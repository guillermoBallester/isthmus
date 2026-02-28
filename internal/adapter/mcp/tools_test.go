package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"io"
	"log/slog"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock SchemaExplorer ---

type mockExplorer struct {
	schemas   []port.SchemaInfo
	tables    []port.TableInfo
	detail    *port.TableDetail
	discovery *port.DiscoveryResult
	err       error
}

func (m *mockExplorer) ListSchemas(_ context.Context) ([]port.SchemaInfo, error) {
	return m.schemas, m.err
}

func (m *mockExplorer) ListTables(_ context.Context) ([]port.TableInfo, error) {
	return m.tables, m.err
}

func (m *mockExplorer) DescribeTable(_ context.Context, _, _ string) (*port.TableDetail, error) {
	return m.detail, m.err
}

func (m *mockExplorer) Discover(_ context.Context) (*port.DiscoveryResult, error) {
	return m.discovery, m.err
}

// --- mock QueryExecutor ---

type mockExecutor struct {
	result  []map[string]any
	err     error
	lastSQL string // captures the SQL passed to Execute
}

func (m *mockExecutor) Execute(_ context.Context, sql string) ([]map[string]any, error) {
	m.lastSQL = sql
	return m.result, m.err
}

// --- helpers ---

func callTool(t *testing.T, s *server.MCPServer, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx := context.Background()
	session := server.NewInProcessSession("test", nil)
	require.NoError(t, s.RegisterSession(ctx, session))
	sessionCtx := s.WithContext(ctx, session)

	// Initialize session.
	initBytes, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "init", "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
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

func toolText(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}

func setupServer(explorer *mockExplorer, executor *mockExecutor) *server.MCPServer {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var querySvc *service.QueryService
	if executor != nil {
		querySvc = service.NewQueryService(domain.NewPgQueryValidator(), executor, port.NoopAuditor{}, logger, nil, nil, nil)
	}

	s := server.NewMCPServer("test", "0.1.0", server.WithToolCapabilities(true))
	RegisterTools(s, explorer, querySvc, logger)
	return s
}

// --- tests ---

func TestDiscover_HappyPath(t *testing.T) {
	explorer := &mockExplorer{
		discovery: &port.DiscoveryResult{
			Schemas: []port.SchemaOverview{
				{
					Name: "public",
					Tables: []port.TableInfo{
						{Schema: "public", Name: "users", Type: "table", RowEstimate: 100},
					},
				},
			},
		},
	}
	s := setupServer(explorer, nil)

	result := callTool(t, s, "discover", nil)
	text := toolText(result)

	var discovery port.DiscoveryResult
	require.NoError(t, json.Unmarshal([]byte(text), &discovery))
	require.Len(t, discovery.Schemas, 1)
	assert.Equal(t, "public", discovery.Schemas[0].Name)
	require.Len(t, discovery.Schemas[0].Tables, 1)
	assert.Equal(t, "users", discovery.Schemas[0].Tables[0].Name)
}

func TestDiscover_Error(t *testing.T) {
	explorer := &mockExplorer{err: fmt.Errorf("permission denied")}
	s := setupServer(explorer, nil)

	result := callTool(t, s, "discover", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "internal error")
}

func TestDescribeTable_HappyPath(t *testing.T) {
	explorer := &mockExplorer{
		detail: &port.TableDetail{
			Schema:      "public",
			Name:        "users",
			RowEstimate: 1000,
			Columns: []port.ColumnInfo{
				{Name: "id", DataType: "uuid", IsPrimaryKey: true},
				{Name: "email", DataType: "text", Stats: &port.ColumnStats{
					Cardinality:   domain.CardinalityUnique,
					NullFraction:  0.01,
					DistinctCount: 1000,
				}},
			},
		},
	}
	s := setupServer(explorer, nil)

	result := callTool(t, s, "describe_table", map[string]any{"table_name": "users"})
	text := toolText(result)

	var detail port.TableDetail
	require.NoError(t, json.Unmarshal([]byte(text), &detail))
	assert.Equal(t, "users", detail.Name)
	assert.Len(t, detail.Columns, 2)
	assert.Equal(t, int64(1000), detail.RowEstimate)
	assert.NotNil(t, detail.Columns[1].Stats)
	assert.Equal(t, domain.CardinalityUnique, detail.Columns[1].Stats.Cardinality)
}

func TestDescribeTable_MissingTableName(t *testing.T) {
	s := setupServer(&mockExplorer{}, nil)

	result := callTool(t, s, "describe_table", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "table_name is required")
}

func TestDescribeTable_Error(t *testing.T) {
	explorer := &mockExplorer{err: fmt.Errorf("table not found")}
	s := setupServer(explorer, nil)

	result := callTool(t, s, "describe_table", map[string]any{"table_name": "nonexistent"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "internal error")
}

func TestQuery_HappyPath(t *testing.T) {
	executor := &mockExecutor{
		result: []map[string]any{{"id": 1, "name": "alice"}},
	}
	s := setupServer(&mockExplorer{}, executor)

	result := callTool(t, s, "query", map[string]any{"sql": "SELECT id, name FROM users"})
	text := toolText(result)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "alice", rows[0]["name"])
}

func TestQuery_MissingSQL(t *testing.T) {
	s := setupServer(&mockExplorer{}, &mockExecutor{})

	result := callTool(t, s, "query", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "sql is required")
}

func TestQuery_ExecutorError(t *testing.T) {
	executor := &mockExecutor{err: fmt.Errorf("connection timeout")}
	s := setupServer(&mockExplorer{}, executor)

	result := callTool(t, s, "query", map[string]any{"sql": "SELECT 1"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "internal error")
}

func TestQuery_WithExplain(t *testing.T) {
	executor := &mockExecutor{
		result: []map[string]any{{"QUERY PLAN": "Seq Scan on users"}},
	}
	s := setupServer(&mockExplorer{}, executor)

	result := callTool(t, s, "query", map[string]any{
		"sql":     "SELECT id FROM users",
		"explain": true,
	})
	assert.False(t, result.IsError)
	assert.Equal(t, "EXPLAIN SELECT id FROM users", executor.lastSQL)
}

func TestQuery_WithExplainAnalyze(t *testing.T) {
	executor := &mockExecutor{
		result: []map[string]any{{"QUERY PLAN": "Seq Scan on users (actual time=0.01..0.02 rows=1)"}},
	}
	s := setupServer(&mockExplorer{}, executor)

	result := callTool(t, s, "query", map[string]any{
		"sql":     "SELECT id FROM users",
		"explain": true,
		"analyze": true,
	})
	assert.False(t, result.IsError)
	assert.Equal(t, "EXPLAIN ANALYZE SELECT id FROM users", executor.lastSQL)
}

func TestQuery_ValidationErrorPassthrough(t *testing.T) {
	executor := &mockExecutor{}
	s := setupServer(&mockExplorer{}, executor)

	result := callTool(t, s, "query", map[string]any{"sql": "DROP TABLE users"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "only SELECT queries are allowed")
}

// --- sanitizeError tests ---

func TestSanitizeError_ValidationPassthrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"empty query", domain.ErrEmptyQuery, "empty query"},
		{"not allowed", domain.ErrNotAllowed, "only SELECT"},
		{"multi statement", domain.ErrMultiStatement, "multiple statements"},
		{"parse error", fmt.Errorf("%w: syntax error", domain.ErrParseFailed), "failed to parse SQL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := sanitizeError(logger, tt.err, "query")
			assert.Contains(t, msg, tt.contains)
		})
	}
}

func TestSanitizeError_Timeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	msg := sanitizeError(logger, context.DeadlineExceeded, "query")
	assert.Contains(t, msg, "query timed out")

	pgErr := &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}
	msg = sanitizeError(logger, pgErr, "query")
	assert.Contains(t, msg, "query timed out")
}

func TestSanitizeError_Generic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	msg := sanitizeError(logger, fmt.Errorf("unexpected pg error: relation OID 12345"), "describe table")
	assert.Contains(t, msg, "internal error")
	assert.Contains(t, msg, "check server logs")
	assert.NotContains(t, msg, "OID")
}
