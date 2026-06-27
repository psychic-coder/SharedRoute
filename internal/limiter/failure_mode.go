package limiter

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/psychic-coder/shardroute/internal/metrics"
)

type FailureMode string

const (
	FailOpen   FailureMode = "fail_open"
	FailClosed FailureMode = "fail_closed"
)

type RedisChecker interface {
	Ping(ctx context.Context) error
}

type FailureHandler struct {
	mu                 sync.Mutex
	mode               FailureMode
	unhealthyThreshold int
	checkInterval      time.Duration
	consecutiveFails   int
	degraded          bool
	lastHealthy       time.Time
	stopCh            chan struct{}
}

func NewFailureHandler(mode FailureMode, threshold int, checkInterval time.Duration) *FailureHandler {
	return &FailureHandler{
		mode:               mode,
		unhealthyThreshold: threshold,
		checkInterval:      checkInterval,
		lastHealthy:        time.Now(),
		stopCh:             make(chan struct{}),
	}
}

func (f *FailureHandler) StartHealthCheck(ctx context.Context, checker RedisChecker) {
	go func() {
		ticker := time.NewTicker(f.checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-f.stopCh:
				return
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(ctx, f.checkInterval/2)
				err := checker.Ping(pingCtx)
				cancel()
				f.RecordRedisResult(err)
			}
		}
	}()
}

func (f *FailureHandler) Stop() {
	close(f.stopCh)
}

func (f *FailureHandler) RecordRedisResult(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		f.consecutiveFails = 0
		if f.degraded {
			f.degraded = false
			metrics.NodeHealth.Set(1)
		}
		f.lastHealthy = time.Now()
		return
	}
	f.consecutiveFails++
	if f.consecutiveFails >= f.unhealthyThreshold {
		if !f.degraded {
			f.degraded = true
			metrics.NodeHealth.Set(0)
		}
	}
}

func (f *FailureHandler) IsDegraded() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.degraded
}

func (f *FailureHandler) HandleRedisError(err error, localAllowed bool) (bool, error) {
	if err == nil {
		return localAllowed, nil
	}
	metrics.RedisUnavailable.Inc()
	f.RecordRedisResult(err)
	if f.mode == FailOpen {
		return localAllowed, nil
	}
	return false, errors.New("rate limit backend unavailable")
}
