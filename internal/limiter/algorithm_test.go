package limiter

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) add(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

func TestTokenBucketRefill(t *testing.T) {
	c := &fakeClock{t: time.UnixMilli(0)}
	l := NewLimiter(c)
	cfg := LimitConfig{Capacity: 10, RefillRatePerSecond: 5, WindowSizeMillis: 1000, MaxRequestsPerWindow: 100, MaxRingEntries: 50}

	for i := 0; i < 10; i++ {
		res := l.Check("k", 1, cfg)
		require.True(t, res.Allowed)
	}
	res := l.Check("k", 1, cfg)
	require.False(t, res.Allowed)

	c.add(2 * time.Second)
	res = l.Check("k", 1, cfg)
	require.True(t, res.Allowed)
}

func TestSlidingWindowBoundaryReject(t *testing.T) {
	c := &fakeClock{t: time.UnixMilli(0)}
	l := NewLimiter(c)
	cfg := LimitConfig{Capacity: 100, RefillRatePerSecond: 100, WindowSizeMillis: 1000, MaxRequestsPerWindow: 3, MaxRingEntries: 50}

	require.True(t, l.Check("k", 1, cfg).Allowed)
	require.True(t, l.Check("k", 1, cfg).Allowed)
	require.True(t, l.Check("k", 1, cfg).Allowed)

	c.add(999 * time.Millisecond)
	res := l.Check("k", 1, cfg)
	require.False(t, res.Allowed)
}

func TestRingBufferWraps(t *testing.T) {
	c := &fakeClock{t: time.UnixMilli(0)}
	l := NewLimiter(c)
	cfg := LimitConfig{Capacity: 100, RefillRatePerSecond: 100, WindowSizeMillis: 1000, MaxRequestsPerWindow: 100, MaxRingEntries: 2}

	st := l.getState("k", cfg, 0)
	ptr := &st.window.values[0]
	l.Check("k", 1, cfg)
	c.add(1 * time.Millisecond)
	l.Check("k", 1, cfg)
	c.add(1 * time.Millisecond)
	l.Check("k", 1, cfg)
	require.Equal(t, ptr, &st.window.values[0])
}

func TestConcurrentAccessNoRace(t *testing.T) {
	c := &fakeClock{t: time.UnixMilli(0)}
	l := NewLimiter(c)
	cfg := LimitConfig{Capacity: 1000, RefillRatePerSecond: 1000, WindowSizeMillis: 1000, MaxRequestsPerWindow: 1000, MaxRingEntries: 50}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.Check("k", 1, cfg)
		}()
	}
	wg.Wait()
}
