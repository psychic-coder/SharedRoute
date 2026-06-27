package config

import (
	"os"
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
	cfg.Redis.Addrs = []string{}
	require.Error(t, cfg.Validate())
}

func TestConfigEnvOverride(t *testing.T) {
	os.Setenv("SHARDBROUTE_HTTP_PORT", "9999")
	os.Setenv("SHARDBROUTE_FAILURE_MODE", "fail_closed")
	defer os.Unsetenv("SHARDBROUTE_HTTP_PORT")
	defer os.Unsetenv("SHARDBROUTE_FAILURE_MODE")

	cfg, err := Load("")
	require.NoError(t, err)
	require.Equal(t, 9999, cfg.Server.HTTPPort)
	require.Equal(t, "fail_closed", cfg.FailureMode)
}
