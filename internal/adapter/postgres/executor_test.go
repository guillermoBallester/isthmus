package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/guillermoBallester/isthmus/internal/adapter/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_Explain(t *testing.T) {
	pool := setupTestDB(t)
	executor := postgres.NewExecutor(pool, true, 100, 10*time.Second)
	ctx := context.Background()

	results, err := executor.Execute(ctx, "EXPLAIN SELECT * FROM customers")
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestExecute_Select_RowLimit(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()

	// Insert 5 rows
	for i := 0; i < 5; i++ {
		_, err := pool.Exec(ctx, "INSERT INTO customers (name, email) VALUES ($1, $2)",
			"user", nil)
		require.NoError(t, err)
	}

	executor := postgres.NewExecutor(pool, true, 3, 10*time.Second)

	results, err := executor.Execute(ctx, "SELECT id, name FROM customers")
	require.NoError(t, err)
	assert.Len(t, results, 3, "should be limited to maxRows=3")
}

func TestExecute_StatementTimeout(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()

	// Use a 1-second timeout; pg_sleep(30) should be cancelled by statement_timeout.
	executor := postgres.NewExecutor(pool, true, 100, 1*time.Second)

	_, err := executor.Execute(ctx, "SELECT pg_sleep(30)")
	require.Error(t, err)

	// PostgreSQL cancels with SQLSTATE 57014 (query_canceled), or the Go
	// context expires first ("context deadline exceeded" / "timeout").
	errMsg := strings.ToLower(err.Error())
	assert.True(t,
		strings.Contains(errMsg, "statement timeout") ||
			strings.Contains(errMsg, "cancel") ||
			strings.Contains(errMsg, "57014") ||
			strings.Contains(errMsg, "deadline exceeded") ||
			strings.Contains(errMsg, "timeout"),
		"expected timeout-related error, got: %s", err,
	)
}
