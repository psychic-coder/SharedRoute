package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		HTTPPort int `yaml:"http_port"`
		GRPCPort int `yaml:"grpc_port"`
	} `yaml:"server"`
	Redis struct {
		Mode             string   `yaml:"mode"`
		Addrs            []string `yaml:"addrs"`
		DialTimeoutMS     int      `yaml:"dial_timeout_ms"`
		CommandTimeoutMS  int      `yaml:"command_timeout_ms"`
	} `yaml:"redis"`
	Limiter struct {
		DefaultCapacity         float64 `yaml:"default_capacity"`
		DefaultRefillPerSecond  float64 `yaml:"default_refill_per_second"`
		WindowSizeMS            int64   `yaml:"window_size_ms"`
		MaxRequestsPerWindow    int     `yaml:"max_requests_per_window"`
		SyncIntervalMS          int     `yaml:"sync_interval_ms"`
	} `yaml:"limiter"`
	FailureMode string `yaml:"failure_mode"`
	Health struct {
		UnhealthyThreshold int `yaml:"unhealthy_threshold"`
		CheckIntervalMS    int `yaml:"check_interval_ms"`
	} `yaml:"health"`
	Observability struct {
		LogLevel        string `yaml:"log_level"`
		MetricsEnabled   bool   `yaml:"metrics_enabled"`
	} `yaml:"observability"`
}

func Default() Config {
	var c Config
	c.Server.HTTPPort = 8080
	c.Server.GRPCPort = 9090
	c.Redis.Mode = "single"
	c.Redis.Addrs = []string{"localhost:6379"}
	c.Redis.DialTimeoutMS = 200
	c.Redis.CommandTimeoutMS = 100
	c.Limiter.DefaultCapacity = 100
	c.Limiter.DefaultRefillPerSecond = 10
	c.Limiter.WindowSizeMS = 1000
	c.Limiter.MaxRequestsPerWindow = 100
	c.Limiter.SyncIntervalMS = 100
	c.FailureMode = "fail_open"
	c.Health.UnhealthyThreshold = 3
	c.Health.CheckIntervalMS = 2000
	c.Observability.LogLevel = "info"
	c.Observability.MetricsEnabled = true
	return c
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}
	overrideInt(&cfg.Server.HTTPPort, "SHARDBROUTE_HTTP_PORT")
	overrideInt(&cfg.Server.GRPCPort, "SHARDBROUTE_GRPC_PORT")
	if v := os.Getenv("SHARDBROUTE_REDIS_ADDRS"); v != "" { cfg.Redis.Addrs = strings.Split(v, ",") }
	overrideString(&cfg.Redis.Mode, "SHARDBROUTE_REDIS_MODE")
	overrideInt(&cfg.Redis.DialTimeoutMS, "SHARDBROUTE_REDIS_DIAL_TIMEOUT_MS")
	overrideInt(&cfg.Redis.CommandTimeoutMS, "SHARDBROUTE_REDIS_COMMAND_TIMEOUT_MS")
	overrideFloat(&cfg.Limiter.DefaultCapacity, "SHARDBROUTE_LIMITER_DEFAULT_CAPACITY")
	overrideFloat(&cfg.Limiter.DefaultRefillPerSecond, "SHARDBROUTE_LIMITER_DEFAULT_REFILL_PER_SECOND")
	overrideInt64(&cfg.Limiter.WindowSizeMS, "SHARDBROUTE_LIMITER_WINDOW_SIZE_MS")
	overrideInt(&cfg.Limiter.MaxRequestsPerWindow, "SHARDBROUTE_LIMITER_MAX_REQUESTS_PER_WINDOW")
	overrideInt(&cfg.Limiter.SyncIntervalMS, "SHARDBROUTE_LIMITER_SYNC_INTERVAL_MS")
	overrideString(&cfg.FailureMode, "SHARDBROUTE_FAILURE_MODE")
	overrideInt(&cfg.Health.UnhealthyThreshold, "SHARDBROUTE_HEALTH_UNHEALTHY_THRESHOLD")
	overrideInt(&cfg.Health.CheckIntervalMS, "SHARDBROUTE_HEALTH_CHECK_INTERVAL_MS")
	overrideString(&cfg.Observability.LogLevel, "SHARDBROUTE_LOG_LEVEL")
	overrideBool(&cfg.Observability.MetricsEnabled, "SHARDBROUTE_METRICS_ENABLED")
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.FailureMode != "fail_open" && c.FailureMode != "fail_closed" {
		return errors.New("invalid failure_mode")
	}
	if c.Limiter.SyncIntervalMS == 0 {
		return errors.New("sync_interval_ms must be > 0")
	}
	if len(c.Redis.Addrs) == 0 || c.Redis.Addrs[0] == "" {
		return errors.New("redis.addrs must not be empty")
	}
	return nil
}

func overrideString(dst *string, k string) { if v := os.Getenv(k); v != "" { *dst = v } }
func overrideInt(dst *int, k string) { if v := os.Getenv(k); v != "" { if n, err := strconv.Atoi(v); err == nil { *dst = n } } }
func overrideInt64(dst *int64, k string) { if v := os.Getenv(k); v != "" { if n, err := strconv.ParseInt(v, 10, 64); err == nil { *dst = n } } }
func overrideFloat(dst *float64, k string) { if v := os.Getenv(k); v != "" { if n, err := strconv.ParseFloat(v, 64); err == nil { *dst = n } } }
func overrideBool(dst *bool, k string) { if v := os.Getenv(k); v != "" { if n, err := strconv.ParseBool(v); err == nil { *dst = n } } }
