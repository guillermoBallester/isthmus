package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)

	assert.Equal(t, "postgres://localhost/test", cfg.DatabaseURL)
	assert.True(t, cfg.ReadOnly)
	assert.Equal(t, 100, cfg.MaxRows)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	_, err := Load(Overrides{})
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

	cfg, err := Load(Overrides{})
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

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "READ_ONLY")
}

func TestLoad_InvalidMaxRows(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("MAX_ROWS", "-1")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAX_ROWS")
}

func TestLoad_InvalidQueryTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("QUERY_TIMEOUT", "not-a-duration")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "QUERY_TIMEOUT")
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

func TestLoad_CLIOverridesEnvVars(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://env-host/db")
	t.Setenv("MAX_ROWS", "100")
	t.Setenv("LOG_LEVEL", "info")

	dsn := "postgres://cli-host/db"
	maxRows := 250
	timeout := 45 * time.Second
	logLevel := "debug"
	policyFile := "/etc/isthmus/policy.yaml"

	cfg, err := Load(Overrides{
		DatabaseURL:  &dsn,
		MaxRows:      &maxRows,
		QueryTimeout: &timeout,
		LogLevel:     &logLevel,
		PolicyFile:   &policyFile,
		DryRun:       true,
		ExplainOnly:  true,
		AuditLog:     "/tmp/audit.jsonl",
	})
	require.NoError(t, err)

	assert.Equal(t, "postgres://cli-host/db", cfg.DatabaseURL)
	assert.Equal(t, 250, cfg.MaxRows)
	assert.Equal(t, 45*time.Second, cfg.QueryTimeout)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, "/etc/isthmus/policy.yaml", cfg.PolicyFile)
	assert.True(t, cfg.DryRun)
	assert.True(t, cfg.ExplainOnly)
	assert.Equal(t, "/tmp/audit.jsonl", cfg.AuditLog)
}

func TestLoad_DatabaseURLFromFlag(t *testing.T) {
	// No DATABASE_URL env var set — flag provides it.
	dsn := "postgres://flag-host/db"
	cfg, err := Load(Overrides{DatabaseURL: &dsn})
	require.NoError(t, err)
	assert.Equal(t, "postgres://flag-host/db", cfg.DatabaseURL)
}
