package limiter

import (
	"context"
	"sync"
	"time"
)

type ApproximateCounter struct {
	Tokens    float64
	LastSync  time.Time
	LastTouch time.Time
}

type LocalCache struct {
	mu     sync.RWMutex
	items  map[string]*ApproximateCounter
	store  *RedisStore
	cfg    LimitConfig
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewLocalCache(store *RedisStore, cfg LimitConfig) *LocalCache {
	return &LocalCache{
		items:  make(map[string]*ApproximateCounter),
		store:  store,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

func (c *LocalCache) Touch(key string) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok {
		c.items[key] = &ApproximateCounter{Tokens: c.cfg.Capacity, LastSync: now, LastTouch: now}
		return
	}
	c.items[key].LastTouch = now
}

func (c *LocalCache) CheckLocal(key string, cost float64) (bool, float64) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	ac, ok := c.items[key]
	if !ok {
		ac = &ApproximateCounter{Tokens: c.cfg.Capacity, LastSync: now, LastTouch: now}
		c.items[key] = ac
	}
	if ac.Tokens >= cost {
		ac.Tokens -= cost
		ac.LastTouch = now
		return true, ac.Tokens
	}
	return false, ac.Tokens
}

// Bounded-overrun tradeoff: requests may be admitted optimistically between syncs,
// then corrected by the authoritative Redis reconciliation on the next sync tick.
func (c *LocalCache) StartSync(ctx context.Context) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(time.Duration(c.cfg.SyncIntervalMS) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.syncOnce(ctx)
			}
		}
	}()
}

func (c *LocalCache) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *LocalCache) syncOnce(ctx context.Context) {
	c.mu.Lock()
	now := time.Now()
	evictThreshold := 5 * time.Minute
	
	var toEvict []string
	keys := make([]string, 0, len(c.items))
	
	for k, ac := range c.items {
		if now.Sub(ac.LastTouch) > evictThreshold {
			toEvict = append(toEvict, k)
		} else {
			keys = append(keys, k)
		}
	}
	
	for _, k := range toEvict {
		delete(c.items, k)
	}
	c.mu.Unlock()

	for _, k := range keys {
		res, err := c.store.CheckAndDecrement(ctx, k, c.cfg, 1) // Note: this currently adds 1 to the Redis window
		if err == nil {
			c.mu.Lock()
			if ac, ok := c.items[k]; ok {
				ac.Tokens = res.TokensRemaining
				ac.LastSync = time.Now()
			}
			c.mu.Unlock()
		}
	}
}

func (c *LocalCache) Snapshot(key string) (ApproximateCounter, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ac, ok := c.items[key]
	if !ok {
		return ApproximateCounter{}, false
	}
	return *ac, true
}
