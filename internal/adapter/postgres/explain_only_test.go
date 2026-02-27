package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type capturingExecutor struct {
	lastSQL string
}

func (c *capturingExecutor) Execute(_ context.Context, sql string) ([]map[string]any, error) {
	c.lastSQL = sql
	return nil, nil
}

func TestExplainOnlyExecutor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expectedSQL string
	}{
		{"plain SELECT gets EXPLAIN prefix", "SELECT 1", "EXPLAIN SELECT 1"},
		{"EXPLAIN is passed through", "EXPLAIN SELECT 1", "EXPLAIN SELECT 1"},
		{"EXPLAIN ANALYZE is passed through", "EXPLAIN ANALYZE SELECT 1", "EXPLAIN ANALYZE SELECT 1"},
		{"lowercase explain is passed through", "explain SELECT 1", "explain SELECT 1"},
		{"mixed case explain is passed through", "Explain SELECT 1", "Explain SELECT 1"},
		{"leading whitespace SELECT", "  SELECT 1", "EXPLAIN   SELECT 1"},
		{"leading whitespace EXPLAIN", "  EXPLAIN SELECT 1", "  EXPLAIN SELECT 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inner := &capturingExecutor{}
			e := NewExplainOnlyExecutor(inner)

			_, _ = e.Execute(context.Background(), tt.input)
			assert.Equal(t, tt.expectedSQL, inner.lastSQL)
		})
	}
}
