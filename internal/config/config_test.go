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

func TestLoad_MaxRowsZero(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("MAX_ROWS", "0")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAX_ROWS")
}

func TestLoad_MaxRowsNonNumeric(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("MAX_ROWS", "abc")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAX_ROWS")
}

func TestLoad_MaxRowsOverrideZero(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	maxRows := 0
	_, err := Load(Overrides{MaxRows: &maxRows})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max-rows")
}

func TestLoad_MaxRowsOverrideNegative(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	maxRows := -5
	_, err := Load(Overrides{MaxRows: &maxRows})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max-rows")
}

func TestLoad_SchemasWithEmptySegments(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("SCHEMAS", "public,,app, ,sales")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, []string{"public", "app", "sales"}, cfg.Schemas)
}

func TestLoad_SchemasTrailingComma(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("SCHEMAS", "public,")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, []string{"public"}, cfg.Schemas)
}

func TestLoad_LogLevelWarning(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "warning")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, slog.LevelWarn, cfg.LogLevel)
}

func TestLoad_LogLevelCaseInsensitive(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
}

func TestLoad_LogLevelOverrideInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	level := "verbose"
	_, err := Load(Overrides{LogLevel: &level})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)

	assert.True(t, cfg.ReadOnly)
	assert.Equal(t, 100, cfg.MaxRows)
	assert.Equal(t, 10*time.Second, cfg.QueryTimeout)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
	assert.Empty(t, cfg.Schemas)
	assert.Empty(t, cfg.PolicyFile)
	assert.False(t, cfg.DryRun)
	assert.False(t, cfg.ExplainOnly)
	assert.Empty(t, cfg.AuditLog)
	assert.Equal(t, "stdio", cfg.Transport)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.False(t, cfg.OTelEnabled)
}

func TestLoad_QueryTimeoutOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	timeout := 500 * time.Millisecond
	cfg, err := Load(Overrides{QueryTimeout: &timeout})
	require.NoError(t, err)
	assert.Equal(t, 500*time.Millisecond, cfg.QueryTimeout)
}

func TestLoad_TransportDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, "stdio", cfg.Transport)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
}

func TestLoad_TransportHTTP(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSPORT", "http")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("HTTP_BEARER_TOKEN", "secret")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, "http", cfg.Transport)
	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, "secret", cfg.HTTPBearerToken)
}

func TestLoad_TransportInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSPORT", "grpc")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TRANSPORT")
}

func TestLoad_TransportOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	transport := "http"
	httpAddr := ":3000"
	token := "my-token"
	cfg, err := Load(Overrides{Transport: &transport, HTTPAddr: &httpAddr, HTTPBearerToken: &token})
	require.NoError(t, err)
	assert.Equal(t, "http", cfg.Transport)
	assert.Equal(t, ":3000", cfg.HTTPAddr)
	assert.Equal(t, "my-token", cfg.HTTPBearerToken)
}

func TestLoad_OTelEnabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("OTEL_ENABLED", "true")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.True(t, cfg.OTelEnabled)
}

func TestLoad_OTelEnabledOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load(Overrides{OTelEnabled: true})
	require.NoError(t, err)
	assert.True(t, cfg.OTelEnabled)
}

func TestLoad_OTelEnabledInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("OTEL_ENABLED", "nope")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OTEL_ENABLED")
}

// --- Bearer token tests ---

func TestLoad_HTTPTransportRequiresToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSPORT", "http")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP_BEARER_TOKEN")
}

func TestLoad_StdioIgnoresMissingToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, "stdio", cfg.Transport)
	assert.Empty(t, cfg.HTTPBearerToken)
}

func TestLoad_HTTPBearerTokenCLIOverridesEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSPORT", "http")
	t.Setenv("HTTP_BEARER_TOKEN", "env-token")

	token := "cli-token"
	cfg, err := Load(Overrides{HTTPBearerToken: &token})
	require.NoError(t, err)
	assert.Equal(t, "cli-token", cfg.HTTPBearerToken)
}

// --- Pool config tests ---

func TestLoad_PoolDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, int32(5), cfg.PoolMaxConns)
	assert.Equal(t, int32(1), cfg.PoolMinConns)
	assert.Equal(t, 30*time.Minute, cfg.PoolMaxConnLifetime)
}

func TestLoad_PoolEnvVars(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POOL_MAX_CONNS", "10")
	t.Setenv("POOL_MIN_CONNS", "2")
	t.Setenv("POOL_MAX_CONN_LIFETIME", "1h")

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	assert.Equal(t, int32(10), cfg.PoolMaxConns)
	assert.Equal(t, int32(2), cfg.PoolMinConns)
	assert.Equal(t, time.Hour, cfg.PoolMaxConnLifetime)
}

func TestLoad_PoolInvalidMaxConns(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POOL_MAX_CONNS", "-1")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "POOL_MAX_CONNS")
}

func TestLoad_PoolInvalidMinConns(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POOL_MIN_CONNS", "-1")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "POOL_MIN_CONNS")
}

func TestLoad_PoolMinExceedsMax(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POOL_MAX_CONNS", "2")
	t.Setenv("POOL_MIN_CONNS", "5")

	_, err := Load(Overrides{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "POOL_MIN_CONNS")
}

func TestLoad_PoolCLIOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	maxConns := int32(20)
	minConns := int32(3)
	lifetime := 45 * time.Minute
	cfg, err := Load(Overrides{
		PoolMaxConns:        &maxConns,
		PoolMinConns:        &minConns,
		PoolMaxConnLifetime: &lifetime,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(20), cfg.PoolMaxConns)
	assert.Equal(t, int32(3), cfg.PoolMinConns)
	assert.Equal(t, 45*time.Minute, cfg.PoolMaxConnLifetime)
}
