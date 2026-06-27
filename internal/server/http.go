package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/psychic-coder/shardroute/internal/limiter"
	"github.com/psychic-coder/shardroute/internal/metrics"
)

type HTTPServer struct {
	Store   *limiter.RedisStore
	Cache   *limiter.LocalCache
	Fail    *limiter.FailureHandler
	Log     *slog.Logger
	Config  limiter.LimitConfig
}

type checkReq struct {
	Key       string  `json:"key"`
	Cost      float64 `json:"cost"`
	LimitName string  `json:"limit_name"`
}

type checkResp struct {
	Allowed         bool    `json:"allowed"`
	TokensRemaining  float64 `json:"tokens_remaining"`
	RetryAfterMillis int64   `json:"retry_after_ms,omitempty"`
	Error           string  `json:"error,omitempty"`
}

func (s *HTTPServer) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/check", s.check)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.Fail != nil && s.Fail.IsDegraded() { w.WriteHeader(http.StatusServiceUnavailable); return }
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/metrics", promhttp.Handler())
	return RequestIDMiddleware(mux, s.Log)
}

func (s *HTTPServer) check(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	start := time.Now()
	defer func() { metrics.RequestDuration.Observe(time.Since(start).Seconds()) }()

	var req checkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" || req.Cost < 0 || req.LimitName == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if s.Cache != nil {
		if allowed, rem := s.Cache.CheckLocal(req.Key, req.Cost); allowed {
			metrics.LocalCacheHit.Inc()
			metrics.RequestsTotal.WithLabelValues("allowed").Inc()
			_ = json.NewEncoder(w).Encode(checkResp{Allowed: true, TokensRemaining: rem})
			return
		}
		metrics.LocalCacheMiss.Inc()
	}

	res, err := s.Store.CheckAndDecrement(r.Context(), req.Key, s.Config, req.Cost)
	if err != nil && s.Fail != nil {
		allowed, failErr := s.Fail.HandleRedisError(err, false)
		if failErr != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(checkResp{Allowed: false, Error: failErr.Error()})
			return
		}
		res.Allowed = allowed
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(checkResp{Allowed: false, Error: "internal error"})
		return
	}

	if res.Allowed {
		metrics.RequestsTotal.WithLabelValues("allowed").Inc()
		w.WriteHeader(http.StatusOK)
	} else {
		metrics.RequestsTotal.WithLabelValues("rejected").Inc()
		w.WriteHeader(http.StatusTooManyRequests)
	}
	_ = json.NewEncoder(w).Encode(checkResp{
		Allowed:         res.Allowed,
		TokensRemaining:  res.TokensRemaining,
		RetryAfterMillis: res.RetryAfterMillis,
	})
}
