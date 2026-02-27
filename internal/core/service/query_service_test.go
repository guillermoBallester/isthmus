package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/guillermoBallester/isthmus/internal/audit"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- mock QueryExecutor ---

type mockExecutor struct {
	executeCalled bool
	lastSQL       string
	result        []map[string]any
	err           error
}

func (m *mockExecutor) Execute(_ context.Context, sql string) ([]map[string]any, error) {
	m.executeCalled = true
	m.lastSQL = sql
	return m.result, m.err
}

// --- tests ---

func TestQueryService_ValidSelect(t *testing.T) {
	exec := &mockExecutor{
		result: []map[string]any{{"id": 1, "name": "alice"}},
	}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	rows, err := svc.Execute(context.Background(), "SELECT id, name FROM users")
	require.NoError(t, err)
	assert.True(t, exec.executeCalled)
	assert.Equal(t, "SELECT id, name FROM users", exec.lastSQL)
	require.Len(t, rows, 1)
	assert.Equal(t, "alice", rows[0]["name"])
}

func TestQueryService_RejectsInsert(t *testing.T) {
	exec := &mockExecutor{}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	_, err := svc.Execute(context.Background(), "INSERT INTO users (name) VALUES ('bob')")
	require.Error(t, err)
	assert.False(t, exec.executeCalled, "executor should not be called for rejected queries")
}

func TestQueryService_RejectsDrop(t *testing.T) {
	exec := &mockExecutor{}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	_, err := svc.Execute(context.Background(), "DROP TABLE users")
	require.Error(t, err)
	assert.False(t, exec.executeCalled)
}

func TestQueryService_RejectsDelete(t *testing.T) {
	exec := &mockExecutor{}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	_, err := svc.Execute(context.Background(), "DELETE FROM users WHERE id = 1")
	require.Error(t, err)
	assert.False(t, exec.executeCalled)
}

func TestQueryService_RejectsUpdate(t *testing.T) {
	exec := &mockExecutor{}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	_, err := svc.Execute(context.Background(), "UPDATE users SET name = 'x'")
	require.Error(t, err)
	assert.False(t, exec.executeCalled)
}

func TestQueryService_AllowsExplain(t *testing.T) {
	exec := &mockExecutor{
		result: []map[string]any{{"QUERY PLAN": "Seq Scan"}},
	}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	rows, err := svc.Execute(context.Background(), "EXPLAIN SELECT 1")
	require.NoError(t, err)
	assert.True(t, exec.executeCalled)
	require.Len(t, rows, 1)
}

func TestQueryService_ExecutorError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("connection refused")}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	_, err := svc.Execute(context.Background(), "SELECT 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestQueryService_EmptyQuery(t *testing.T) {
	exec := &mockExecutor{}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	_, err := svc.Execute(context.Background(), "")
	require.Error(t, err)
	assert.False(t, exec.executeCalled)
}

func TestQueryService_WithMasks(t *testing.T) {
	exec := &mockExecutor{
		result: []map[string]any{
			{"id": 1, "email": "alice@example.com", "name": "Alice"},
			{"id": 2, "email": "bob@example.com", "name": "Bob"},
		},
	}
	masks := map[string]domain.MaskType{"email": domain.MaskRedact}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), masks)

	rows, err := svc.Execute(context.Background(), "SELECT id, email, name FROM users")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "***", rows[0]["email"])
	assert.Equal(t, "***", rows[1]["email"])
	assert.Equal(t, "Alice", rows[0]["name"])
}

func TestQueryService_NoMasks(t *testing.T) {
	exec := &mockExecutor{
		result: []map[string]any{
			{"id": 1, "email": "alice@example.com"},
		},
	}
	svc := NewQueryService(domain.NewPgQueryValidator(), exec, audit.NoopAuditor{}, testLogger(), nil)

	rows, err := svc.Execute(context.Background(), "SELECT id, email FROM users")
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", rows[0]["email"])
}
