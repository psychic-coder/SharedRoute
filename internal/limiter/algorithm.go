package limiter

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type LimitConfig struct {
	Capacity            float64
	RefillRatePerSecond  float64
	WindowSizeMillis     int64
	MaxRequestsPerWindow int
	MaxRingEntries       int
	SyncIntervalMS       int
}

type Result struct {
	Allowed         bool
	TokensRemaining  float64
	RetryAfterMillis int64
}

type keyState struct {
	tokens     float64
	lastRefill  int64
	window     *ringBuffer
}

type ringBuffer struct {
	values []int64
	start  int
	size   int
}

func newRingBuffer(capacity int) *ringBuffer {
	if capacity <= 0 {
		capacity = 50
	}
	return &ringBuffer{values: make([]int64, capacity)}
}

func (r *ringBuffer) add(v int64) {
	if len(r.values) == 0 {
		return
	}
	if r.size < len(r.values) {
		idx := (r.start + r.size) % len(r.values)
		r.values[idx] = v
		r.size++
		return
	}
	r.values[r.start] = v
	r.start = (r.start + 1) % len(r.values)
}

func (r *ringBuffer) countSince(since int64) int {
	n := 0
	for i := 0; i < r.size; i++ {
		v := r.values[(r.start+i)%len(r.values)]
		if v >= since {
			n++
		}
	}
	return n
}

type Limiter struct {
	mu    sync.Mutex
	clock Clock
	keys  map[string]*keyState
}

func NewLimiter(clock Clock) *Limiter {
	if clock == nil {
		clock = RealClock{}
	}
	return &Limiter{
		clock: clock,
		keys:  make(map[string]*keyState),
	}
}

func (l *Limiter) getState(key string, cfg LimitConfig, now int64) *keyState {
	st, ok := l.keys[key]
	if !ok {
		st = &keyState{
			tokens:    cfg.Capacity,
			lastRefill: now,
			window:    newRingBuffer(cfg.MaxRingEntries),
		}
		l.keys[key] = st
	}
	if st.window == nil {
		st.window = newRingBuffer(cfg.MaxRingEntries)
	}
	return st
}

// A request is allowed only if both the token bucket check and the sliding-window check pass.
func (l *Limiter) Check(key string, cost float64, cfg LimitConfig) Result {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now().UnixMilli()
	st := l.getState(key, cfg, now)

	elapsed := now - st.lastRefill
	if elapsed < 0 {
		elapsed = 0
	}
	refill := (float64(elapsed) / 1000.0) * cfg.RefillRatePerSecond
	tokens := st.tokens + refill
	if tokens > cfg.Capacity {
		tokens = cfg.Capacity
	}

	windowSince := now - cfg.WindowSizeMillis
	windowCount := st.window.countSince(windowSince)

	if tokens < cost || windowCount >= cfg.MaxRequestsPerWindow {
		retry := int64(0)
		if tokens < cost && cfg.RefillRatePerSecond > 0 {
			retry = int64(((cost - tokens) / cfg.RefillRatePerSecond) * 1000)
		} else if windowCount >= cfg.MaxRequestsPerWindow {
			retry = cfg.WindowSizeMillis
		}
		return Result{Allowed: false, TokensRemaining: st.tokens, RetryAfterMillis: retry}
	}

	tokens -= cost
	st.tokens = tokens
	st.lastRefill = now
	st.window.add(now)
	return Result{Allowed: true, TokensRemaining: tokens, RetryAfterMillis: 0}
}
