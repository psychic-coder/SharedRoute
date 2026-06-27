package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "shardroute_requests_total", Help: "Total requests"}, []string{"result"})
	RequestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "shardroute_request_duration_seconds", Help: "Request duration"})
	RedisUnavailable = prometheus.NewCounter(prometheus.CounterOpts{Name: "shardroute_redis_unavailable_total", Help: "Redis unavailable"})
	LocalCacheHit = prometheus.NewCounter(prometheus.CounterOpts{Name: "shardroute_local_cache_hit_total", Help: "Local cache hits"})
	LocalCacheMiss = prometheus.NewCounter(prometheus.CounterOpts{Name: "shardroute_local_cache_miss_total", Help: "Local cache misses"})
	SyncDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "shardroute_sync_duration_seconds", Help: "Sync duration"})
	NodeHealth = prometheus.NewGauge(prometheus.GaugeOpts{Name: "shardroute_node_health_status", Help: "1 healthy, 0 degraded"})
)

func MustRegister() {
	prometheus.MustRegister(RequestsTotal, RequestDuration, RedisUnavailable, LocalCacheHit, LocalCacheMiss, SyncDuration, NodeHealth)
}
