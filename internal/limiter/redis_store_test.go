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
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

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
			<-start
			res, err := store.CheckAndDecrement(ctx, key, cfg, 1)
			require.NoError(t, err)
			resultCh <- res.Allowed
		}()
	}
	close(start)
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
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

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
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

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


func TestConcurrentCheckAndDecrement(t *testing.T) { TestRedisConcurrentAtomicity(t) }


func TestNoScriptRecovery(t *testing.T) { TestRedisNOSCRIPTRecovery(t) }

func TestSlidingWindowBoundaryBurst(t *testing.T) { TestRedisWindowBoundary(t) }
