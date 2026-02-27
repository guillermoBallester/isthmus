package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Database connection.
	DatabaseURL  string
	ReadOnly     bool
	MaxRows      int
	QueryTimeout time.Duration

	// Schema filtering.
	Schemas    []string // empty means all non-system schemas
	PolicyFile string   // optional path to policy YAML

	// Logging.
	LogLevel slog.Level

	// Transport.
	Transport       string // "stdio" (default) or "http"
	HTTPAddr        string // listen address for HTTP transport (default ":8080")
	HTTPBearerToken string // required when transport=http

	// Connection pool.
	PoolMaxConns        int32         // default: 5
	PoolMinConns        int32         // default: 1
	PoolMaxConnLifetime time.Duration // default: 30m

	// Observability.
	OTelEnabled bool // enable OpenTelemetry tracing and metrics

	// CLI-only fields (not settable via env vars).
	DryRun      bool
	ExplainOnly bool
	AuditLog    string // path to NDJSON audit log file
}

// Overrides holds CLI flag values that override environment variables.
// Pointer fields distinguish "not set" from zero values.
type Overrides struct {
	DatabaseURL     *string
	LogLevel        *string
	MaxRows         *int
	QueryTimeout    *time.Duration
	PolicyFile      *string
	Transport       *string
	HTTPAddr        *string
	HTTPBearerToken *string
	OTelEnabled     bool
	DryRun          bool
	ExplainOnly     bool
	AuditLog        string

	// Connection pool overrides.
	PoolMaxConns        *int32
	PoolMinConns        *int32
	PoolMaxConnLifetime *time.Duration
}

// Load builds a Config from environment variables, then applies CLI overrides,
// then validates the result.
func Load(overrides Overrides) (*Config, error) {
	cfg := defaults()

	if err := loadEnvVars(cfg); err != nil {
		return nil, err
	}
	if err := applyOverrides(cfg, overrides); err != nil {
		return nil, err
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// defaults returns a Config populated with default values.
func defaults() *Config {
	return &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		ReadOnly:            true,
		MaxRows:             100,
		QueryTimeout:        10 * time.Second,
		Transport:           "stdio",
		HTTPAddr:            ":8080",
		PoolMaxConns:        5,
		PoolMinConns:        1,
		PoolMaxConnLifetime: 30 * time.Minute,
	}
}

// loadEnvVars reads all supported environment variables into cfg.
func loadEnvVars(cfg *Config) error {
	if v := os.Getenv("READ_ONLY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid READ_ONLY value %q: %w", v, err)
		}
		cfg.ReadOnly = b
	}

	if v := os.Getenv("MAX_ROWS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid MAX_ROWS value %q: must be a positive integer", v)
		}
		cfg.MaxRows = n
	}

	if v := os.Getenv("QUERY_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid QUERY_TIMEOUT value %q: %w", v, err)
		}
		cfg.QueryTimeout = d
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		level, err := parseLogLevel(v)
		if err != nil {
			return err
		}
		cfg.LogLevel = level
	}

	if v := os.Getenv("SCHEMAS"); v != "" {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.Schemas = append(cfg.Schemas, s)
			}
		}
	}

	cfg.PolicyFile = os.Getenv("POLICY_FILE")

	if v := os.Getenv("TRANSPORT"); v != "" {
		cfg.Transport = v
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	cfg.HTTPBearerToken = os.Getenv("HTTP_BEARER_TOKEN")

	if v := os.Getenv("OTEL_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid OTEL_ENABLED value %q: %w", v, err)
		}
		cfg.OTelEnabled = b
	}

	if err := loadPoolEnvVars(cfg); err != nil {
		return err
	}

	return nil
}

// loadPoolEnvVars reads connection pool environment variables.
func loadPoolEnvVars(cfg *Config) error {
	if v := os.Getenv("POOL_MAX_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid POOL_MAX_CONNS value %q: must be a positive integer", v)
		}
		cfg.PoolMaxConns = int32(n)
	}
	if v := os.Getenv("POOL_MIN_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid POOL_MIN_CONNS value %q: must be a non-negative integer", v)
		}
		cfg.PoolMinConns = int32(n)
	}
	if v := os.Getenv("POOL_MAX_CONN_LIFETIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid POOL_MAX_CONN_LIFETIME value %q: %w", v, err)
		}
		cfg.PoolMaxConnLifetime = d
	}
	return nil
}

// applyOverrides applies CLI flag values on top of the env-loaded config.
func applyOverrides(cfg *Config, o Overrides) error {
	if o.DatabaseURL != nil {
		cfg.DatabaseURL = *o.DatabaseURL
	}
	if o.LogLevel != nil {
		level, err := parseLogLevel(*o.LogLevel)
		if err != nil {
			return err
		}
		cfg.LogLevel = level
	}
	if o.MaxRows != nil {
		if *o.MaxRows <= 0 {
			return fmt.Errorf("invalid --max-rows value: must be a positive integer")
		}
		cfg.MaxRows = *o.MaxRows
	}
	if o.QueryTimeout != nil {
		cfg.QueryTimeout = *o.QueryTimeout
	}
	if o.PolicyFile != nil {
		cfg.PolicyFile = *o.PolicyFile
	}
	if o.Transport != nil {
		cfg.Transport = *o.Transport
	}
	if o.HTTPAddr != nil {
		cfg.HTTPAddr = *o.HTTPAddr
	}
	if o.HTTPBearerToken != nil {
		cfg.HTTPBearerToken = *o.HTTPBearerToken
	}

	if err := applyPoolOverrides(cfg, o); err != nil {
		return err
	}

	cfg.DryRun = o.DryRun
	cfg.ExplainOnly = o.ExplainOnly
	cfg.AuditLog = o.AuditLog
	cfg.OTelEnabled = cfg.OTelEnabled || o.OTelEnabled

	return nil
}

// applyPoolOverrides applies connection pool CLI flag overrides.
func applyPoolOverrides(cfg *Config, o Overrides) error {
	if o.PoolMaxConns != nil {
		if *o.PoolMaxConns <= 0 {
			return fmt.Errorf("invalid --pool-max-conns value: must be a positive integer")
		}
		cfg.PoolMaxConns = *o.PoolMaxConns
	}
	if o.PoolMinConns != nil {
		if *o.PoolMinConns < 0 {
			return fmt.Errorf("invalid --pool-min-conns value: must be a non-negative integer")
		}
		cfg.PoolMinConns = *o.PoolMinConns
	}
	if o.PoolMaxConnLifetime != nil {
		cfg.PoolMaxConnLifetime = *o.PoolMaxConnLifetime
	}
	return nil
}

// validate checks cross-field constraints on the final config.
func validate(cfg *Config) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required (set via env var or --database-url flag)")
	}

	switch cfg.Transport {
	case "stdio", "http":
	default:
		return fmt.Errorf("invalid TRANSPORT value %q: must be \"stdio\" or \"http\"", cfg.Transport)
	}

	if cfg.Transport == "http" && cfg.HTTPBearerToken == "" {
		return fmt.Errorf("HTTP_BEARER_TOKEN is required when transport is \"http\" (set via env var or --http-bearer-token flag)")
	}

	if cfg.PoolMinConns > cfg.PoolMaxConns {
		return fmt.Errorf("POOL_MIN_CONNS (%d) must not exceed POOL_MAX_CONNS (%d)", cfg.PoolMinConns, cfg.PoolMaxConns)
	}

	return nil
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid LOG_LEVEL value %q: must be debug, info, warn, or error", s)
	}
}
