package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/guillermoBallester/isthmus/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMiddleware_PanicReturns500(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected error")
	}), logger)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecoveryMiddleware_NoPanicPassesThrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), logger)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, o config.Overrides)
	}{
		{
			name: "no flags",
			args: []string{},
			check: func(t *testing.T, o config.Overrides) {
				assert.False(t, o.DryRun)
				assert.False(t, o.ExplainOnly)
				assert.False(t, o.OTelEnabled)
				assert.Nil(t, o.DatabaseURL)
			},
		},
		{
			name: "dry-run",
			args: []string{"--dry-run"},
			check: func(t *testing.T, o config.Overrides) {
				assert.True(t, o.DryRun)
			},
		},
		{
			name: "explain-only",
			args: []string{"--explain-only"},
			check: func(t *testing.T, o config.Overrides) {
				assert.True(t, o.ExplainOnly)
			},
		},
		{
			name: "database-url",
			args: []string{"--database-url", "postgres://localhost:5432/test"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.DatabaseURL)
				assert.Equal(t, "postgres://localhost:5432/test", *o.DatabaseURL)
			},
		},
		{
			name: "max-rows",
			args: []string{"--max-rows", "500"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.MaxRows)
				assert.Equal(t, 500, *o.MaxRows)
			},
		},
		{
			name: "query-timeout",
			args: []string{"--query-timeout", "45s"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.QueryTimeout)
				assert.Equal(t, 45*time.Second, *o.QueryTimeout)
			},
		},
		{
			name: "transport http with addr and token",
			args: []string{"--transport", "http", "--http-addr", ":9090", "--http-bearer-token", "tok"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.Transport)
				assert.Equal(t, "http", *o.Transport)
				require.NotNil(t, o.HTTPAddr)
				assert.Equal(t, ":9090", *o.HTTPAddr)
				require.NotNil(t, o.HTTPBearerToken)
				assert.Equal(t, "tok", *o.HTTPBearerToken)
			},
		},
		{
			name: "otel",
			args: []string{"--otel"},
			check: func(t *testing.T, o config.Overrides) {
				assert.True(t, o.OTelEnabled)
			},
		},
		{
			name: "pool settings",
			args: []string{"--pool-max-conns", "20", "--pool-min-conns", "2", "--pool-max-conn-lifetime", "1h"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.PoolMaxConns)
				assert.Equal(t, int32(20), *o.PoolMaxConns)
				require.NotNil(t, o.PoolMinConns)
				assert.Equal(t, int32(2), *o.PoolMinConns)
				require.NotNil(t, o.PoolMaxConnLifetime)
				assert.Equal(t, time.Hour, *o.PoolMaxConnLifetime)
			},
		},
		{
			name: "audit-log",
			args: []string{"--audit-log", "/tmp/audit.ndjson"},
			check: func(t *testing.T, o config.Overrides) {
				assert.Equal(t, "/tmp/audit.ndjson", o.AuditLog)
			},
		},
		{
			name: "log-level",
			args: []string{"--log-level", "debug"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.LogLevel)
				assert.Equal(t, "debug", *o.LogLevel)
			},
		},
		{
			name: "policy-file",
			args: []string{"--policy-file", "policy.yaml"},
			check: func(t *testing.T, o config.Overrides) {
				require.NotNil(t, o.PolicyFile)
				assert.Equal(t, "policy.yaml", *o.PolicyFile)
			},
		},
		{
			name:    "unknown flag returns error",
			args:    []string{"--unknown-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overrides, err := parseFlags(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, overrides)
			}
		})
	}
}

func TestRedactDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "with password",
			dsn:  "postgres://user:secret@localhost:5432/mydb",
			want: "postgres://user:%2A%2A%2A@localhost:5432/mydb",
		},
		{
			name: "without password",
			dsn:  "postgres://user@localhost:5432/mydb",
			want: "postgres://user@localhost:5432/mydb",
		},
		{
			name: "invalid dsn",
			dsn:  "://invalid",
			want: "***",
		},
		{
			name: "with query params",
			dsn:  "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			want: "postgres://user:%2A%2A%2A@localhost:5432/mydb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactDSN(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}
