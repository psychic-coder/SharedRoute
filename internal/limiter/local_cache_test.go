package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestLocalCacheStaleState(t *testing.T) {
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

	cfg := LimitConfig{Capacity: 10, SyncIntervalMS: 10}
	cache := NewLocalCache(store, cfg)
	
	syncCtx, cancel := context.WithCancel(context.Background())
	cache.StartSync(syncCtx)
	
	cache.Touch("test")
	time.Sleep(20 * time.Millisecond)

	cancel()
	cache.Stop()

	allowed1, _ := cache.CheckLocal("test", 5)
	require.True(t, allowed1)
	
	allowed2, _ := cache.CheckLocal("test", 5)
	require.True(t, allowed2)
	
	allowed3, _ := cache.CheckLocal("test", 1)
	require.False(t, allowed3)
}

func BenchmarkLocalCacheVSRedis(b *testing.B) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(b, err)
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	require.NoError(b, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(b, err)

	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

	store := NewRedisStore(client)
	require.NoError(b, store.Load(ctx))

	cfg := LimitConfig{Capacity: float64(b.N), SyncIntervalMS: 100}
	cache := NewLocalCache(store, cfg)

	b.Run("LocalCache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.CheckLocal("bench_key_local", 1)
		}
	})

	b.Run("RedisDirect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = store.CheckAndDecrement(ctx, "bench_key_redis", cfg, 1)
		}
	})
}


func BenchmarkLocalCacheHit(b *testing.B) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(b, err)
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	require.NoError(b, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(b, err)

	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

	store := NewRedisStore(client)
	require.NoError(b, store.Load(ctx))


	cfg := LimitConfig{Capacity: 1e9, SyncIntervalMS: 10000}
	cache := NewLocalCache(store, cfg)
	cache.Touch("bench_cache")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.CheckLocal("bench_cache", 1)
		}
	})
}


func BenchmarkRedisDirectHit(b *testing.B) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(b, err)
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	require.NoError(b, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(b, err)

	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

	store := NewRedisStore(client)
	require.NoError(b, store.Load(ctx))


	cfg := LimitConfig{Capacity: 1e9, SyncIntervalMS: 0}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = store.CheckAndDecrement(ctx, "bench_redis", cfg, 1)
		}
	})
}
