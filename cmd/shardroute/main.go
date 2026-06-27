package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/psychic-coder/shardroute/internal/config"
	"github.com/psychic-coder/shardroute/internal/limiter"
	"github.com/psychic-coder/shardroute/internal/metrics"
	"github.com/psychic-coder/shardroute/internal/server"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type redisChecker struct {
	client redis.UniversalClient
}

func (r redisChecker) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func main() {
	cfg, err := config.Load(os.Getenv("SHARDBROUTE_CONFIG"))
	if err != nil {
		panic(err)
	}
	if cfg.Observability.MetricsEnabled {
		metrics.MustRegister()
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if cfg.Observability.LogLevel != "info" {
		log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	var redisClient redis.UniversalClient
	if cfg.Redis.Mode == "cluster" {
		redisClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.Redis.Addrs,
			DialTimeout:  time.Duration(cfg.Redis.DialTimeoutMS) * time.Millisecond,
			ReadTimeout:  time.Duration(cfg.Redis.CommandTimeoutMS) * time.Millisecond,
			WriteTimeout: time.Duration(cfg.Redis.CommandTimeoutMS) * time.Millisecond,
		})
	} else {
		redisClient = redis.NewClient(&redis.Options{
			Addr:         cfg.Redis.Addrs[0],
			DialTimeout:  time.Duration(cfg.Redis.DialTimeoutMS) * time.Millisecond,
			ReadTimeout:  time.Duration(cfg.Redis.CommandTimeoutMS) * time.Millisecond,
			WriteTimeout: time.Duration(cfg.Redis.CommandTimeoutMS) * time.Millisecond,
		})
	}
	defer redisClient.Close()

	store := limiter.NewRedisStore(redisClient)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Load(ctx); err != nil {
		log.Error("failed to load lua script", "error", err)
		os.Exit(1)
	}

	limitCfg := limiter.LimitConfig{
		Capacity:             cfg.Limiter.DefaultCapacity,
		RefillRatePerSecond:  cfg.Limiter.DefaultRefillPerSecond,
		WindowSizeMillis:     cfg.Limiter.WindowSizeMS,
		MaxRequestsPerWindow: cfg.Limiter.MaxRequestsPerWindow,
		SyncIntervalMS:       cfg.Limiter.SyncIntervalMS,
	}

	cache := limiter.NewLocalCache(store, limitCfg)
	syncCtx, syncCancel := context.WithCancel(context.Background())
	cache.StartSync(syncCtx)

	failMode := limiter.FailOpen
	if cfg.FailureMode == "fail_closed" {
		failMode = limiter.FailClosed
	}
	failHandler := limiter.NewFailureHandler(failMode, cfg.Health.UnhealthyThreshold, time.Duration(cfg.Health.CheckIntervalMS)*time.Millisecond)
	failHandler.StartHealthCheck(syncCtx, redisChecker{redisClient})

	httpSrv := &server.HTTPServer{
		Store:   store,
		Cache:   cache,
		Fail:    failHandler,
		Log:     log,
		Config:  limitCfg,
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler: httpSrv.Routes(),
	}

	grpcServer := grpc.NewServer()
	server.RegisterGRPC(grpcServer, &server.GRPCServer{
		Store:  store,
		Cache:  cache,
		Fail:   failHandler,
		Config: limitCfg,
	})

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
		if err != nil {
			log.Error("failed to listen for grpc", "error", err)
			return
		}
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server failed", "error", err)
		}
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server failed", "error", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch

	log.Info("shutting down")
	syncCancel()
	cache.Stop()
	failHandler.Stop()
	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
