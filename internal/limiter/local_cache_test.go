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
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

	store := NewRedisStore(client)
	require.NoError(t, store.Load(ctx))

	cfg := LimitConfig{Capacity: 10, SyncIntervalMS: 10}
	cache := NewLocalCache(store, cfg)
	
	syncCtx, cancel := context.WithCancel(context.Background())
	cache.StartSync(syncCtx)
	
	cache.Touch("test")
	time.Sleep(20 * time.Millisecond) // Let it sync once

	// Kill sync goroutine
	cancel()
	cache.Stop()

	// Should still serve from stale state
	allowed1, _ := cache.CheckLocal("test", 5)
	require.True(t, allowed1)
	
	allowed2, _ := cache.CheckLocal("test", 5)
	require.True(t, allowed2)
	
	allowed3, _ := cache.CheckLocal("test", 1)
	require.False(t, allowed3) // tokens exhausted locally
}

func BenchmarkLocalCacheVSRedis(b *testing.B) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(b, err)
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(b, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(b, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

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

// BenchmarkLocalCacheHit measures the pure in-memory local cache path (no network I/O).
// Run with: go test ./internal/limiter/... -bench=BenchmarkLocalCacheHit -benchtime=10s -cpu=1,4,8
func BenchmarkLocalCacheHit(b *testing.B) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(b, err)
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(b, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(b, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

	store := NewRedisStore(client)
	require.NoError(b, store.Load(ctx))

	// Large capacity so the cache never exhausts tokens during the benchmark run.
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

// BenchmarkRedisDirectHit measures the Redis round-trip path (Lua EVALSHA over loopback).
// Run with: go test ./internal/limiter/... -bench=BenchmarkRedisDirectHit -benchtime=10s -cpu=1,4,8
func BenchmarkRedisDirectHit(b *testing.B) {
	ctx := context.Background()
	container, err := testredis.Run(ctx, "redis:7-alpine")
	require.NoError(b, err)
	defer container.Terminate(ctx) //nolint:errcheck

	connStr, err := container.ConnectionString(ctx)
	require.NoError(b, err)

	opts, err := redis.ParseURL(connStr)
	require.NoError(b, err)

	client := redis.NewClient(opts)
	defer client.Close() //nolint:errcheck

	store := NewRedisStore(client)
	require.NoError(b, store.Load(ctx))

	// Large capacity so the limit is never hit during the benchmark.
	cfg := LimitConfig{Capacity: 1e9, SyncIntervalMS: 0}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = store.CheckAndDecrement(ctx, "bench_redis", cfg, 1)
		}
	})
}
