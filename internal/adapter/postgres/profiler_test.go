package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/guillermoBallester/isthmus/internal/adapter/postgres"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testSchemaProfiler creates a richer schema for profiler/explorer testing.
const testSchemaProfiler = `
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

	-- Table with _id column but no explicit FK (for implicit FK inference)
	CREATE TABLE reviews (
		id         SERIAL PRIMARY KEY,
		product_id INTEGER NOT NULL,
		user_id    INTEGER NOT NULL,
		rating     SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
		body       TEXT
	);

	-- Seed data for stats.
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

func setupProfilerDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
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

	_, err = pool.Exec(ctx, testSchemaProfiler)
	require.NoError(t, err)

	// Run ANALYZE to populate pg_stats.
	_, err = pool.Exec(ctx, "ANALYZE")
	require.NoError(t, err)

	return pool
}

func TestDescribeTable_ColumnStats(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	detail, err := explorer.DescribeTable(ctx, "", "products")
	require.NoError(t, err)

	assert.Equal(t, "Product catalog", detail.Comment)
	assert.Greater(t, detail.RowEstimate, int64(0))
	assert.Greater(t, detail.TotalBytes, int64(0))
	assert.NotEmpty(t, detail.SizeHuman)

	// Find the status column — should be enum_like with 3 values.
	var statusCol *port.ColumnInfo
	var priceCol *port.ColumnInfo
	var deletedAtCol *port.ColumnInfo
	for i, col := range detail.Columns {
		switch col.Name {
		case "status":
			statusCol = &detail.Columns[i]
		case "price":
			priceCol = &detail.Columns[i]
		case "deleted_at":
			deletedAtCol = &detail.Columns[i]
		}
	}

	require.NotNil(t, statusCol, "status column not found")
	require.NotNil(t, statusCol.Stats, "status column should have stats")
	assert.Equal(t, domain.CardinalityEnumLike, statusCol.Stats.Cardinality)
	assert.NotEmpty(t, statusCol.Stats.MostCommonVals, "enum-like column should have most common values")
	// Should contain the three status values.
	assert.Contains(t, statusCol.Stats.MostCommonVals, "active")
	assert.NotEmpty(t, statusCol.Stats.MostCommonFreqs, "enum-like column should have frequencies")

	require.NotNil(t, priceCol, "price column not found")
	if priceCol.Stats != nil {
		// Price should have min/max range.
		assert.NotEmpty(t, priceCol.Stats.MinValue, "numeric column should have min value")
		assert.NotEmpty(t, priceCol.Stats.MaxValue, "numeric column should have max value")
	}

	require.NotNil(t, deletedAtCol, "deleted_at column not found")
	if deletedAtCol.Stats != nil {
		// deleted_at should have high null fraction (all NULLs in our seed data).
		assert.Greater(t, deletedAtCol.Stats.NullFraction, 0.9, "deleted_at should be mostly NULL")
	}
}

func TestDescribeTable_CheckConstraints(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	detail, err := explorer.DescribeTable(ctx, "", "products")
	require.NoError(t, err)

	require.NotEmpty(t, detail.CheckConstraints, "products should have check constraints")
	found := false
	for _, ck := range detail.CheckConstraints {
		if ck.Name != "" && ck.Expression != "" {
			found = true
		}
	}
	assert.True(t, found, "should find at least one named check constraint with expression")
}

func TestDescribeTable_StatsAge(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	detail, err := explorer.DescribeTable(ctx, "", "products")
	require.NoError(t, err)

	// We ran ANALYZE, so stats_age should be set.
	assert.NotNil(t, detail.StatsAge, "stats_age should be set after ANALYZE")
	assert.True(t, detail.StatsAge.Before(time.Now()), "stats_age should be in the past")
}

func TestListTables_Enhanced(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	tables, err := explorer.ListTables(ctx)
	require.NoError(t, err)

	tableMap := make(map[string]port.TableInfo)
	for _, tbl := range tables {
		tableMap[tbl.Name] = tbl
	}

	products := tableMap["products"]
	assert.Equal(t, "table", products.Type)
	assert.Greater(t, products.RowEstimate, int64(0))
	assert.Greater(t, products.TotalBytes, int64(0))
	assert.NotEmpty(t, products.SizeHuman)
	assert.Greater(t, products.ColumnCount, 0)
	assert.True(t, products.HasIndexes)

	categories := tableMap["categories"]
	assert.Greater(t, categories.ColumnCount, 0)
}

func TestDescribeTable_SampleRows(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	detail, err := explorer.DescribeTable(ctx, "", "products")
	require.NoError(t, err)

	assert.NotEmpty(t, detail.SampleRows, "should have sample rows")
	assert.LessOrEqual(t, len(detail.SampleRows), 5, "should have at most 5 sample rows")

	for _, row := range detail.SampleRows {
		assert.Contains(t, row, "id")
		assert.Contains(t, row, "name")
		assert.Contains(t, row, "status")
	}
}

func TestDescribeTable_IndexUsage(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	detail, err := explorer.DescribeTable(ctx, "", "products")
	require.NoError(t, err)

	assert.NotEmpty(t, detail.IndexUsage, "products should have index usage stats")

	indexNames := make(map[string]bool)
	for _, u := range detail.IndexUsage {
		indexNames[u.Name] = true
		assert.Greater(t, u.SizeBytes, int64(0), "index %s should have non-zero size", u.Name)
	}

	assert.True(t, indexNames["products_pkey"], "should include primary key index")
	assert.True(t, indexNames["idx_products_category"], "should include category index")
}

func TestDescribeTable_StatsAgeWarning(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	detail, err := explorer.DescribeTable(ctx, "", "products")
	require.NoError(t, err)

	// We just ran ANALYZE, so no warning expected.
	assert.NotNil(t, detail.StatsAge)
	assert.Empty(t, detail.StatsAgeWarning, "should not warn about fresh stats")
}

func TestDiscover(t *testing.T) {
	pool := setupProfilerDB(t)
	explorer := postgres.NewExplorer(pool, nil)
	ctx := context.Background()

	result, err := explorer.Discover(ctx)
	require.NoError(t, err)

	// Find public schema.
	var publicSchema *port.SchemaOverview
	for i, s := range result.Schemas {
		if s.Name == "public" {
			publicSchema = &result.Schemas[i]
			break
		}
	}
	require.NotNil(t, publicSchema, "should contain public schema")

	tableMap := make(map[string]port.TableInfo)
	for _, tbl := range publicSchema.Tables {
		tableMap[tbl.Name] = tbl
	}

	assert.Contains(t, tableMap, "products")
	assert.Contains(t, tableMap, "categories")
	assert.Contains(t, tableMap, "reviews")

	products := tableMap["products"]
	assert.Equal(t, "table", products.Type)
	assert.Greater(t, products.RowEstimate, int64(0))
	assert.Equal(t, 8, products.ColumnCount)
}
