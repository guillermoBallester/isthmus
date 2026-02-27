package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"io"
	"log/slog"

	"github.com/guillermoBallester/isthmus/internal/audit"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock SchemaExplorer ---

type mockExplorer struct {
	schemas []port.SchemaInfo
	tables  []port.TableInfo
	detail  *port.TableDetail
	err     error
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

// --- mock SchemaProfiler ---

type mockProfiler struct {
	profile *port.TableProfile
	err     error
}

func (m *mockProfiler) ProfileTable(_ context.Context, _, _ string) (*port.TableProfile, error) {
	return m.profile, m.err
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

func setupServer(explorer *mockExplorer, profiler *mockProfiler, executor *mockExecutor) *server.MCPServer {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var prof port.SchemaProfiler
	if profiler != nil {
		prof = profiler
	}

	var querySvc *service.QueryService
	if executor != nil {
		querySvc = service.NewQueryService(domain.NewPgQueryValidator(), executor, audit.NoopAuditor{}, logger, nil, nil, nil)
	}

	s := server.NewMCPServer("test", "0.1.0", server.WithToolCapabilities(true))
	RegisterTools(s, explorer, prof, querySvc)
	return s
}

// --- tests ---

func TestListSchemas_HappyPath(t *testing.T) {
	explorer := &mockExplorer{
		schemas: []port.SchemaInfo{{Name: "public"}, {Name: "auth"}},
	}
	s := setupServer(explorer, nil, nil)

	result := callTool(t, s, "list_schemas", nil)
	text := toolText(result)

	var schemas []port.SchemaInfo
	require.NoError(t, json.Unmarshal([]byte(text), &schemas))
	assert.Len(t, schemas, 2)
	assert.Equal(t, "public", schemas[0].Name)
}

func TestListSchemas_Error(t *testing.T) {
	explorer := &mockExplorer{err: fmt.Errorf("permission denied")}
	s := setupServer(explorer, nil, nil)

	result := callTool(t, s, "list_schemas", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "permission denied")
}

func TestListTables_HappyPath(t *testing.T) {
	explorer := &mockExplorer{
		tables: []port.TableInfo{
			{Schema: "public", Name: "users", Type: "table", RowEstimate: 100, ColumnCount: 5, HasIndexes: true},
		},
	}
	s := setupServer(explorer, nil, nil)

	result := callTool(t, s, "list_tables", nil)
	text := toolText(result)

	var tables []port.TableInfo
	require.NoError(t, json.Unmarshal([]byte(text), &tables))
	require.Len(t, tables, 1)
	assert.Equal(t, "users", tables[0].Name)
	assert.Equal(t, 5, tables[0].ColumnCount)
	assert.True(t, tables[0].HasIndexes)
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
	s := setupServer(explorer, nil, nil)

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
	s := setupServer(&mockExplorer{}, nil, nil)

	result := callTool(t, s, "describe_table", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "table_name is required")
}

func TestDescribeTable_Error(t *testing.T) {
	explorer := &mockExplorer{err: fmt.Errorf("table not found")}
	s := setupServer(explorer, nil, nil)

	result := callTool(t, s, "describe_table", map[string]any{"table_name": "nonexistent"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "table not found")
}

func TestProfileTable_HappyPath(t *testing.T) {
	profiler := &mockProfiler{
		profile: &port.TableProfile{
			Schema:      "public",
			Name:        "orders",
			RowEstimate: 50000,
			TotalBytes:  4194304,
			TableBytes:  3145728,
			IndexBytes:  1048576,
			SizeHuman:   "4096 kB",
			IndexUsage: []port.IndexUsage{
				{Name: "orders_pkey", Scans: 10000, SizeBytes: 524288, SizeHuman: "512 kB"},
			},
			InferredFKs: []port.InferredFK{
				{ColumnName: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id", Confidence: "high"},
			},
		},
	}
	s := setupServer(&mockExplorer{}, profiler, nil)

	result := callTool(t, s, "profile_table", map[string]any{"table_name": "orders"})
	text := toolText(result)

	var profile port.TableProfile
	require.NoError(t, json.Unmarshal([]byte(text), &profile))
	assert.Equal(t, "orders", profile.Name)
	assert.Equal(t, int64(50000), profile.RowEstimate)
	assert.Len(t, profile.IndexUsage, 1)
	assert.Len(t, profile.InferredFKs, 1)
	assert.Equal(t, "customer_id", profile.InferredFKs[0].ColumnName)
}

func TestProfileTable_MissingTableName(t *testing.T) {
	profiler := &mockProfiler{}
	s := setupServer(&mockExplorer{}, profiler, nil)

	result := callTool(t, s, "profile_table", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "table_name is required")
}

func TestProfileTable_Error(t *testing.T) {
	profiler := &mockProfiler{err: fmt.Errorf("table not found")}
	s := setupServer(&mockExplorer{}, profiler, nil)

	result := callTool(t, s, "profile_table", map[string]any{"table_name": "nonexistent"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "table not found")
}

func TestQuery_HappyPath(t *testing.T) {
	executor := &mockExecutor{
		result: []map[string]any{{"id": 1, "name": "alice"}},
	}
	s := setupServer(&mockExplorer{}, nil, executor)

	result := callTool(t, s, "query", map[string]any{"sql": "SELECT id, name FROM users"})
	text := toolText(result)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "alice", rows[0]["name"])
}

func TestQuery_MissingSQL(t *testing.T) {
	s := setupServer(&mockExplorer{}, nil, &mockExecutor{})

	result := callTool(t, s, "query", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "sql is required")
}

func TestQuery_ExecutorError(t *testing.T) {
	executor := &mockExecutor{err: fmt.Errorf("connection timeout")}
	s := setupServer(&mockExplorer{}, nil, executor)

	result := callTool(t, s, "query", map[string]any{"sql": "SELECT 1"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "connection timeout")
}

func TestExplainQuery_HappyPath(t *testing.T) {
	executor := &mockExecutor{
		result: []map[string]any{{"QUERY PLAN": "Seq Scan on users"}},
	}
	s := setupServer(&mockExplorer{}, nil, executor)

	result := callTool(t, s, "explain_query", map[string]any{"sql": "SELECT id FROM users"})
	text := toolText(result)

	assert.Equal(t, "EXPLAIN SELECT id FROM users", executor.lastSQL)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "Seq Scan on users", rows[0]["QUERY PLAN"])
}

func TestExplainQuery_WithAnalyze(t *testing.T) {
	executor := &mockExecutor{
		result: []map[string]any{{"QUERY PLAN": "Seq Scan on users (actual time=0.01..0.02 rows=1)"}},
	}
	s := setupServer(&mockExplorer{}, nil, executor)

	result := callTool(t, s, "explain_query", map[string]any{
		"sql":     "SELECT id FROM users",
		"analyze": true,
	})
	assert.False(t, result.IsError)
	assert.Equal(t, "EXPLAIN ANALYZE SELECT id FROM users", executor.lastSQL)
}

func TestExplainQuery_MissingSQL(t *testing.T) {
	s := setupServer(&mockExplorer{}, nil, &mockExecutor{})

	result := callTool(t, s, "explain_query", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(result), "sql is required")
}
