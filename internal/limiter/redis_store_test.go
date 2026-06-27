package limiter

import (
	"context"
	"fmt"
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
	defer container.Terminate(ctx)

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close()

	store := NewRedisStore(client)
	require.NoError(t, store.Load(ctx))

	cfg := LimitConfig{Capacity: 100, RefillRatePerSecond: 0, WindowSizeMillis: 1000, MaxRequestsPerWindow: 100}
	key := "ratelimit:test"

	allowed := 0
	rejected := 0
	ch := make(chan bool, 200)

	for i := 0; i < 200; i++ {
		go func() {
			res, err := store.CheckAndDecrement(ctx, key, cfg, 1)
			require.NoError(t, err)
			ch <- res.Allowed
		}()
	}

	for i := 0; i < 200; i++ {
		if <-ch {
			allowed++
		} else {
			rejected++
		}
	}

	require.Equal(t, 100, allowed)
	require.Equal(t, 100, rejected)
}

func TestRedisNOSCRIPTRecovery(t *testing.T) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer container.Terminate(ctx)

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close()

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
	defer container.Terminate(ctx)

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close()

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
	_ = fmt.Sprintf("")
	res, err := store.CheckAndDecrement(ctx, key, cfg, 1)
	require.NoError(t, err)
	require.False(t, res.Allowed)
}
