package postgres

import (
	"context"

	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// ExplainOnlyExecutor wraps a QueryExecutor and forces all queries through EXPLAIN.
// Non-EXPLAIN queries are automatically prefixed with "EXPLAIN ".
type ExplainOnlyExecutor struct {
	inner port.QueryExecutor
}

func NewExplainOnlyExecutor(inner port.QueryExecutor) *ExplainOnlyExecutor {
	return &ExplainOnlyExecutor{inner: inner}
}

func (e *ExplainOnlyExecutor) Execute(ctx context.Context, sql string) ([]map[string]any, error) {
	if !isExplain(sql) {
		sql = "EXPLAIN " + sql
	}
	return e.inner.Execute(ctx, sql)
}
