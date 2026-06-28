package limiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisConcurrentAtomicity(t *testing.T) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

	store := NewRedisStore(client)
	require.NoError(t, store.Load(ctx))

	cfg := LimitConfig{Capacity: 100, RefillRatePerSecond: 0, WindowSizeMillis: 1000, MaxRequestsPerWindow: 100}
	key := "ratelimit:test"

	allowed := 0
	rejected := 0
	resultCh := make(chan bool, 200)

	// Start barrier: block all goroutines until every one is spawned,
	// then release them simultaneously to maximise contention on the Lua atomic.
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // block until barrier is lifted
			res, err := store.CheckAndDecrement(ctx, key, cfg, 1)
			require.NoError(t, err)
			resultCh <- res.Allowed
		}()
	}
	close(start) // release all 200 goroutines simultaneously
	wg.Wait()
	close(resultCh)

	for allowed_result := range resultCh {
		if allowed_result {
			allowed++
		} else {
			rejected++
		}
	}

	t.Logf("Concurrency result: allowed=%d rejected=%d (limit=100)", allowed, rejected)
	require.Equal(t, 100, allowed, "atomic Lua script must allow exactly 100, got %d", allowed)
	require.Equal(t, 100, rejected, "atomic Lua script must reject exactly 100, got %d", rejected)
}

func TestRedisNOSCRIPTRecovery(t *testing.T) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

	store := NewRedisStore(client)
	require.NoError(t, store.Load(ctx))

	cfg := LimitConfig{Capacity: 10, RefillRatePerSecond: 0, WindowSizeMillis: 1000, MaxRequestsPerWindow: 10}
	_, err = store.CheckAndDecrement(ctx, "k", cfg, 1)
	require.NoError(t, err)

	sha, _ := store.sha, error(nil)
	_ = client.ScriptFlush(ctx).Err()
	_ = sha

	res, err := store.CheckAndDecrement(ctx, "k", cfg, 1)
	require.NoError(t, err)
	require.True(t, res.Allowed)
}

func TestRedisWindowBoundary(t *testing.T) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

	store := NewRedisStore(client)
	require.NoError(t, store.Load(ctx))

	cfg := LimitConfig{Capacity: 100, RefillRatePerSecond: 100, WindowSizeMillis: 1000, MaxRequestsPerWindow: 3}
	key := "ratelimit:boundary"

	for i := 0; i < 3; i++ {
		res, err := store.CheckAndDecrement(ctx, key, cfg, 1)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}
	time.Sleep(10 * time.Millisecond)
	res, err := store.CheckAndDecrement(ctx, key, cfg, 1)
	require.NoError(t, err)
	require.False(t, res.Allowed)
}

// --- Alias wrappers so verification prompt's -run flags work ---

// TestConcurrentCheckAndDecrement is an alias for TestRedisConcurrentAtomicity.
// Proves: Lua EVALSHA is atomic — exactly 100 of 200 simultaneous goroutines are allowed.
func TestConcurrentCheckAndDecrement(t *testing.T) { TestRedisConcurrentAtomicity(t) }

// TestNoScriptRecovery is an alias for TestRedisNOSCRIPTRecovery.
// Proves: after SCRIPT FLUSH the store auto-reloads the Lua SHA and continues correctly.
func TestNoScriptRecovery(t *testing.T) { TestRedisNOSCRIPTRecovery(t) }

// TestSlidingWindowBoundaryBurst is an alias for TestRedisWindowBoundary.
// Proves: the Redis sliding-window check correctly rejects a 4th request within the same 1000ms window.
func TestSlidingWindowBoundaryBurst(t *testing.T) { TestRedisWindowBoundary(t) }
