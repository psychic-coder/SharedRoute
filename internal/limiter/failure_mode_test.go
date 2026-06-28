package limiter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockChecker struct {
	mu  sync.Mutex
	err error
}

func (m *mockChecker) Ping(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

func (m *mockChecker) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func TestFailureModeFailOpen(t *testing.T) {
	f := NewFailureHandler(FailOpen, 3, 10*time.Millisecond)

	allowed, err := f.HandleRedisError(errors.New("timeout"), true)
	require.NoError(t, err)
	require.True(t, allowed)

	require.False(t, f.IsDegraded())

	_, _ = f.HandleRedisError(errors.New("timeout"), true)
	_, _ = f.HandleRedisError(errors.New("timeout"), true)

	require.True(t, f.IsDegraded())
}

func TestFailureModeFailClosed(t *testing.T) {
	f := NewFailureHandler(FailClosed, 3, 10*time.Millisecond)

	allowed, err := f.HandleRedisError(errors.New("timeout"), true)
	require.Error(t, err)
	require.False(t, allowed)
	require.Equal(t, "rate limit backend unavailable", err.Error())
}

func TestFailureModeFlappingPrevention(t *testing.T) {
	f := NewFailureHandler(FailOpen, 3, 10*time.Millisecond)

	f.RecordRedisResult(errors.New("timeout"))
	f.RecordRedisResult(nil)
	f.RecordRedisResult(errors.New("timeout"))
	f.RecordRedisResult(errors.New("timeout"))
	require.False(t, f.IsDegraded())

	f.RecordRedisResult(errors.New("timeout"))
	require.True(t, f.IsDegraded())
}

func TestFailureModeBackgroundRecovery(t *testing.T) {
	f := NewFailureHandler(FailOpen, 3, 10*time.Millisecond)
	checker := &mockChecker{err: errors.New("down")}

	f.RecordRedisResult(errors.New("timeout"))
	f.RecordRedisResult(errors.New("timeout"))
	f.RecordRedisResult(errors.New("timeout"))
	require.True(t, f.IsDegraded())

	ctx, cancel := context.WithCancel(context.Background())
	f.StartHealthCheck(ctx, checker)

	time.Sleep(50 * time.Millisecond)
	require.True(t, f.IsDegraded())

	checker.setErr(nil)
	time.Sleep(50 * time.Millisecond)
	
	require.False(t, f.IsDegraded())
	
	cancel()
	f.Stop()
}


func TestFailOpen(t *testing.T) { TestFailureModeFailOpen(t) }


func TestFailClosed(t *testing.T) { TestFailureModeFailClosed(t) }


func TestFlappingPrevention(t *testing.T) { TestFailureModeFlappingPrevention(t) }
