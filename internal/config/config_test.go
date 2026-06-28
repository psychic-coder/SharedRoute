package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigDefaultValidates(t *testing.T) {
	cfg := Default()
	require.NoError(t, cfg.Validate())
}

func TestConfigValidationFailures(t *testing.T) {
	cfg := Default()
	cfg.FailureMode = "invalid"
	require.Error(t, cfg.Validate())

	cfg = Default()
	cfg.Limiter.SyncIntervalMS = 0
	require.Error(t, cfg.Validate())

	cfg = Default()
	cfg.Limiter.CacheMode = "direct"
	cfg.Limiter.SyncIntervalMS = 0
	require.NoError(t, cfg.Validate())

	cfg = Default()
	cfg.Limiter.CacheMode = "turbo"
	require.Error(t, cfg.Validate())

	cfg = Default()
	cfg.Redis.Addrs = []string{}
	require.Error(t, cfg.Validate())
}

func TestConfigEnvOverride(t *testing.T) {
	t.Setenv("SHARDBROUTE_HTTP_PORT", "9999")
	t.Setenv("SHARDBROUTE_FAILURE_MODE", "fail_closed")

	cfg, err := Load("")
	require.NoError(t, err)
	require.Equal(t, 9999, cfg.Server.HTTPPort)
	require.Equal(t, "fail_closed", cfg.FailureMode)
}
