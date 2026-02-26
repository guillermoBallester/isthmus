package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "postgres://localhost/test", cfg.DatabaseURL)
	assert.True(t, cfg.ReadOnly)
	assert.Equal(t, 100, cfg.MaxRows)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("READ_ONLY", "false")
	t.Setenv("MAX_ROWS", "500")
	t.Setenv("QUERY_TIMEOUT", "30s")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SCHEMAS", "public, app")
	t.Setenv("POLICY_FILE", "/tmp/policy.yaml")

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.ReadOnly)
	assert.Equal(t, 500, cfg.MaxRows)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, []string{"public", "app"}, cfg.Schemas)
	assert.Equal(t, "/tmp/policy.yaml", cfg.PolicyFile)
}

func TestLoad_InvalidReadOnly(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("READ_ONLY", "nope")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "READ_ONLY")
}

func TestLoad_InvalidMaxRows(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("MAX_ROWS", "-1")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAX_ROWS")
}

func TestLoad_InvalidQueryTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("QUERY_TIMEOUT", "not-a-duration")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "QUERY_TIMEOUT")
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}
