package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/psychic-coder/shardroute/internal/lua"
	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...any) *redis.Cmd
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
	ScriptLoad(ctx context.Context, script string) *redis.StringCmd
	Ping(ctx context.Context) *redis.StatusCmd
}

type RedisStore struct {
	client RedisClient
	sha    string
}

func NewRedisStore(client RedisClient) *RedisStore {
	return &RedisStore{client: client}
}

func (r *RedisStore) Load(ctx context.Context) error {
	sha, err := r.client.ScriptLoad(ctx, lua.CheckAndDecrementScript).Result()
	if err != nil {
		return err
	}
	r.sha = sha
	return nil
}

func (r *RedisStore) CheckAndDecrement(ctx context.Context, key string, cfg LimitConfig, cost float64) (Result, error) {
	if r.sha == "" {
		if err := r.Load(ctx); err != nil {
			return Result{}, err
		}
	}
	now := time.Now().UnixMilli()
	args := []any{now, cfg.Capacity, cfg.RefillRatePerSecond, cost, cfg.WindowSizeMillis, cfg.MaxRequestsPerWindow}
	call := func() (any, error) {
		return r.client.EvalSha(ctx, r.sha, []string{key}, args...).Result()
	}

	res, err := call()
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "noscript") {
		if err2 := r.Load(ctx); err2 != nil {
			return Result{}, err2
		}
		res, err = call()
	}
	if err != nil {
		return Result{}, err
	}

	arr, ok := res.([]any)
	if !ok || len(arr) < 3 {
		return Result{}, fmt.Errorf("unexpected redis lua response: %T", res)
	}

	allowed := fmt.Sprint(arr[0]) == "1"
	var tokens float64
	fmt.Sscan(fmt.Sprint(arr[1]), &tokens)
	var retry int64
	fmt.Sscan(fmt.Sprint(arr[2]), &retry)
	return Result{Allowed: allowed, TokensRemaining: tokens, RetryAfterMillis: retry}, nil
}
