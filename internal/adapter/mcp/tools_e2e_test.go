package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guillermoBallester/isthmus/internal/adapter/postgres"
	"github.com/guillermoBallester/isthmus/internal/audit"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const e2eSchema = `
	CREATE TABLE categories (
		id   SERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE
	);
	COMMENT ON TABLE categories IS 'Product categories';

	CREATE TABLE products (
		id          SERIAL PRIMARY KEY,
		category_id INTEGER NOT NULL REFERENCES categories(id),
		name        TEXT NOT NULL,
		status      TEXT NOT NULL CHECK (status IN ('active', 'inactive', 'discontinued')),
		price       NUMERIC(10,2) NOT NULL DEFAULT 0,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
		deleted_at  TIMESTAMPTZ,
		metadata    JSONB
	);
	CREATE INDEX idx_products_category ON products(category_id);
	CREATE INDEX idx_products_status ON products(status);
	CREATE INDEX idx_products_created ON products(created_at);
	COMMENT ON TABLE products IS 'Product catalog';
	COMMENT ON COLUMN products.status IS 'Product lifecycle status';

	CREATE TABLE reviews (
		id         SERIAL PRIMARY KEY,
		product_id INTEGER NOT NULL,
		user_id    INTEGER NOT NULL,
		rating     SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
		body       TEXT
	);

	CREATE VIEW active_products AS
		SELECT id, name, price FROM products WHERE status = 'active';

	-- Seed data.
	INSERT INTO categories (name) VALUES ('Electronics'), ('Books'), ('Clothing');

	INSERT INTO products (category_id, name, status, price, created_at)
	SELECT
		(i % 3) + 1,
		'Product ' || i,
		CASE (i % 5)
			WHEN 0 THEN 'inactive'
			WHEN 4 THEN 'discontinued'
			ELSE 'active'
		END,
		(random() * 100)::numeric(10,2),
		now() - (i || ' days')::interval
	FROM generate_series(1, 100) AS i;

	INSERT INTO reviews (product_id, user_id, rating, body)
	SELECT
		(i % 100) + 1,
		(i % 20) + 1,
		(i % 5) + 1,
		CASE WHEN i % 3 = 0 THEN NULL ELSE 'Review ' || i END
	FROM generate_series(1, 200) AS i;
`

// setupE2E starts a Postgres testcontainer, applies the schema, runs ANALYZE,
// and returns a fully wired MCP server backed by real adapters.
func setupE2E(t *testing.T) *server.MCPServer {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	_, err = pool.Exec(ctx, e2eSchema)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "ANALYZE")
	require.NoError(t, err)

	// Real adapters.
	explorer := postgres.NewExplorer(pool, nil)
	profiler := postgres.NewProfiler(pool, nil)
	executor := postgres.NewExecutor(pool, true, 100, 10*time.Second)

	// Real services.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	querySvc := service.NewQueryService(domain.NewPgQueryValidator(), executor, audit.NoopAuditor{}, logger)

	// Real MCP server.
	s := server.NewMCPServer("test-e2e", "0.0.1", server.WithToolCapabilities(true))
	RegisterTools(s, explorer, profiler, querySvc)
	return s
}

func TestE2E_MCPTools(t *testing.T) {
	s := setupE2E(t)

	t.Run("list_schemas", func(t *testing.T) {
		result := callToolE2E(t, s, "list_schemas", nil)
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var schemas []port.SchemaInfo
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &schemas))

		names := make(map[string]bool)
		for _, s := range schemas {
			names[s.Name] = true
		}
		assert.True(t, names["public"], "should contain 'public' schema")
		assert.False(t, names["pg_catalog"], "should exclude pg_catalog")
		assert.False(t, names["information_schema"], "should exclude information_schema")
	})

	t.Run("list_tables", func(t *testing.T) {
		result := callToolE2E(t, s, "list_tables", nil)
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var tables []port.TableInfo
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &tables))

		tableMap := make(map[string]port.TableInfo)
		for _, tbl := range tables {
			tableMap[tbl.Name] = tbl
		}

		assert.Len(t, tables, 4, "expected 3 tables + 1 view")

		products := tableMap["products"]
		assert.Equal(t, "table", products.Type)
		assert.Greater(t, products.RowEstimate, int64(0))
		assert.Greater(t, products.TotalBytes, int64(0))
		assert.Equal(t, 8, products.ColumnCount)
		assert.True(t, products.HasIndexes)
		assert.Equal(t, "Product catalog", products.Comment)

		active := tableMap["active_products"]
		assert.Equal(t, "view", active.Type)
	})

	t.Run("describe_table", func(t *testing.T) {
		result := callToolE2E(t, s, "describe_table", map[string]any{"table_name": "products"})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var detail port.TableDetail
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &detail))

		assert.Equal(t, "public", detail.Schema)
		assert.Equal(t, "products", detail.Name)
		assert.Equal(t, "Product catalog", detail.Comment)
		assert.Len(t, detail.Columns, 8)
		assert.Greater(t, detail.RowEstimate, int64(0))

		// Column map for targeted assertions.
		colMap := make(map[string]port.ColumnInfo)
		for _, c := range detail.Columns {
			colMap[c.Name] = c
		}

		// Primary key.
		assert.True(t, colMap["id"].IsPrimaryKey)

		// Foreign key: category_id -> categories.id.
		require.NotEmpty(t, detail.ForeignKeys)
		fkFound := false
		for _, fk := range detail.ForeignKeys {
			if fk.ColumnName == "category_id" && fk.ReferencedTable == "categories" && fk.ReferencedColumn == "id" {
				fkFound = true
			}
		}
		assert.True(t, fkFound, "should have FK category_id -> categories.id")

		// Indexes (pkey + 3 explicit).
		assert.GreaterOrEqual(t, len(detail.Indexes), 4)

		// Check constraint on status.
		require.NotEmpty(t, detail.CheckConstraints)
		ckFound := false
		for _, ck := range detail.CheckConstraints {
			if containsSubstring(ck.Expression, "status") {
				ckFound = true
			}
		}
		assert.True(t, ckFound, "should have check constraint referencing 'status'")

		// Column stats: status = enum_like.
		statusCol := colMap["status"]
		require.NotNil(t, statusCol.Stats, "status column should have stats")
		assert.Equal(t, domain.CardinalityEnumLike, statusCol.Stats.Cardinality)
		assert.Contains(t, statusCol.Stats.MostCommonVals, "active")

		// Column stats: deleted_at high null fraction.
		deletedAt := colMap["deleted_at"]
		require.NotNil(t, deletedAt.Stats, "deleted_at column should have stats")
		assert.Greater(t, deletedAt.Stats.NullFraction, 0.9)

		// Column stats: price has min/max.
		priceCol := colMap["price"]
		if priceCol.Stats != nil {
			assert.NotEmpty(t, priceCol.Stats.MinValue)
			assert.NotEmpty(t, priceCol.Stats.MaxValue)
		}

		// Stats age should be set (we ran ANALYZE).
		assert.NotNil(t, detail.StatsAge)
	})

	t.Run("describe_table/schema_arg", func(t *testing.T) {
		result := callToolE2E(t, s, "describe_table", map[string]any{
			"table_name": "products",
			"schema":     "public",
		})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var detail port.TableDetail
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &detail))
		assert.Equal(t, "public", detail.Schema)
		assert.Equal(t, "products", detail.Name)
	})

	t.Run("describe_table/not_found", func(t *testing.T) {
		result := callToolE2E(t, s, "describe_table", map[string]any{"table_name": "nonexistent_table"})
		assert.True(t, result.IsError)
		assert.Contains(t, toolText(result), "nonexistent_table")
	})

	t.Run("profile_table", func(t *testing.T) {
		result := callToolE2E(t, s, "profile_table", map[string]any{"table_name": "products"})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var profile port.TableProfile
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &profile))

		assert.Greater(t, profile.TableBytes, int64(0))
		assert.Greater(t, profile.IndexBytes, int64(0))
		assert.GreaterOrEqual(t, profile.TotalBytes, profile.TableBytes+profile.IndexBytes)

		// Sample rows.
		assert.NotEmpty(t, profile.SampleRows)
		for _, row := range profile.SampleRows {
			assert.Contains(t, row, "id")
			assert.Contains(t, row, "name")
			assert.Contains(t, row, "status")
		}

		// Index usage.
		indexNames := make(map[string]bool)
		for _, u := range profile.IndexUsage {
			indexNames[u.Name] = true
		}
		assert.True(t, indexNames["products_pkey"], "should include products_pkey")
		assert.True(t, indexNames["idx_products_category"], "should include idx_products_category")
	})

	t.Run("profile_table/inferred_fks", func(t *testing.T) {
		result := callToolE2E(t, s, "profile_table", map[string]any{"table_name": "reviews"})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var profile port.TableProfile
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &profile))

		fkMap := make(map[string]port.InferredFK)
		for _, fk := range profile.InferredFKs {
			fkMap[fk.ColumnName] = fk
		}

		productFK, ok := fkMap["product_id"]
		require.True(t, ok, "should infer product_id FK")
		assert.Equal(t, "products", productFK.ReferencedTable)
		assert.Equal(t, "id", productFK.ReferencedColumn)
		assert.Equal(t, "high", productFK.Confidence)
	})

	t.Run("profile_table/stats_freshness", func(t *testing.T) {
		result := callToolE2E(t, s, "profile_table", map[string]any{"table_name": "products"})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var profile port.TableProfile
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &profile))

		assert.NotNil(t, profile.StatsAge, "StatsAge should be set after ANALYZE")
		assert.Empty(t, profile.StatsAgeWarning, "should not warn about fresh stats")
	})

	t.Run("query", func(t *testing.T) {
		result := callToolE2E(t, s, "query", map[string]any{
			"sql": "SELECT p.name, c.name AS category FROM products p JOIN categories c ON c.id = p.category_id LIMIT 3",
		})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &rows))
		require.Len(t, rows, 3)
		assert.Contains(t, rows[0], "name")
		assert.Contains(t, rows[0], "category")
	})

	t.Run("query/rejects_insert", func(t *testing.T) {
		result := callToolE2E(t, s, "query", map[string]any{
			"sql": "INSERT INTO categories (name) VALUES ('test')",
		})
		assert.True(t, result.IsError)
		text := toolText(result)
		assert.Contains(t, text, "only SELECT queries are allowed")
	})

	t.Run("explain_query", func(t *testing.T) {
		result := callToolE2E(t, s, "explain_query", map[string]any{
			"sql": "SELECT id FROM products WHERE status = 'active'",
		})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &rows))
		require.NotEmpty(t, rows)
		assert.Contains(t, rows[0], "QUERY PLAN")
	})

	t.Run("explain_query/analyze", func(t *testing.T) {
		result := callToolE2E(t, s, "explain_query", map[string]any{
			"sql":     "SELECT id FROM products WHERE status = 'active'",
			"analyze": true,
		})
		require.False(t, result.IsError, "unexpected error: %s", toolText(result))

		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(toolText(result)), &rows))
		require.NotEmpty(t, rows)
		// EXPLAIN ANALYZE includes "actual time" or "actual rows" in the plan output.
		planText, _ := rows[0]["QUERY PLAN"].(string)
		assert.Contains(t, planText, "actual", "EXPLAIN ANALYZE should include actual timing")
	})
}

var e2eSessionCounter atomic.Int64

// callToolE2E is like callTool but uses a unique session ID per call,
// allowing multiple calls against the same MCP server without "session already exists" errors.
func callToolE2E(t *testing.T, s *server.MCPServer, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx := context.Background()
	sessionID := fmt.Sprintf("e2e-%d", e2eSessionCounter.Add(1))
	session := server.NewInProcessSession(sessionID, nil)
	require.NoError(t, s.RegisterSession(ctx, session))
	sessionCtx := s.WithContext(ctx, session)

	// Initialize session.
	initBytes, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "init", "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test-e2e", "version": "1.0"},
		},
	})
	s.HandleMessage(sessionCtx, initBytes)

	// Call tool.
	reqBytes, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "call-1", "method": "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	})
	resp := s.HandleMessage(sessionCtx, reqBytes)
	respBytes, _ := json.Marshal(resp)

	var rpc struct {
		Result *mcp.CallToolResult       `json:"result"`
		Error  *struct{ Message string } `json:"error,omitempty"`
	}
	require.NoError(t, json.Unmarshal(respBytes, &rpc))
	require.Nil(t, rpc.Error, "unexpected RPC error: %v", rpc.Error)
	require.NotNil(t, rpc.Result)
	return rpc.Result
}

// containsSubstring checks if s contains substr (case-insensitive).
func containsSubstring(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
