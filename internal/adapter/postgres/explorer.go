package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Explorer struct {
	pool    *pgxpool.Pool
	schemas []string // empty means all non-system schemas
}

func NewExplorer(pool *pgxpool.Pool, schemas []string) *Explorer {
	return &Explorer{pool: pool, schemas: schemas}
}

func (e *Explorer) ListSchemas(ctx context.Context) ([]port.SchemaInfo, error) {
	filter, args := schemaFilter(e.schemas, "s.schema_name", 1)
	query := fmt.Sprintf(queryListSchemas, filter)

	rows, err := e.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing schemas: %w", err)
	}
	defer rows.Close()

	var schemas []port.SchemaInfo
	for rows.Next() {
		var s port.SchemaInfo
		if err := rows.Scan(&s.Name); err != nil {
			return nil, fmt.Errorf("scanning schema row: %w", err)
		}
		schemas = append(schemas, s)
	}
	return schemas, rows.Err()
}

func (e *Explorer) ListTables(ctx context.Context) ([]port.TableInfo, error) {
	filter, args := schemaFilter(e.schemas, "t.table_schema", 1)
	query := fmt.Sprintf(queryListTables, filter)

	rows, err := e.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()

	var tables []port.TableInfo
	for rows.Next() {
		var t port.TableInfo
		if err := rows.Scan(
			&t.Schema, &t.Name, &t.Type, &t.RowEstimate,
			&t.TotalBytes, &t.SizeHuman, &t.ColumnCount, &t.HasIndexes,
			&t.Comment,
		); err != nil {
			return nil, fmt.Errorf("scanning table row: %w", err)
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (e *Explorer) Discover(ctx context.Context) (*port.DiscoveryResult, error) {
	schemas, err := e.ListSchemas(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering schemas: %w", err)
	}

	tables, err := e.ListTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables: %w", err)
	}

	// Group tables by schema.
	bySchema := make(map[string][]port.TableInfo)
	for _, t := range tables {
		bySchema[t.Schema] = append(bySchema[t.Schema], t)
	}

	result := &port.DiscoveryResult{
		Schemas: make([]port.SchemaOverview, 0, len(schemas)),
	}
	for _, s := range schemas {
		result.Schemas = append(result.Schemas, port.SchemaOverview{
			Name:   s.Name,
			Tables: bySchema[s.Name],
		})
	}

	return result, nil
}

func (e *Explorer) DescribeTable(ctx context.Context, schema, tableName string) (*port.TableDetail, error) {
	detail := &port.TableDetail{Name: tableName}

	var err error
	if schema != "" {
		detail.Schema = schema
		detail.Comment, err = e.fetchTableComment(ctx, schema, tableName)
	} else {
		detail.Schema, detail.Comment, err = e.fetchTableMeta(ctx, tableName)
	}
	if err != nil {
		return nil, err
	}

	// Fetch table size and row estimate from pg_class.
	detail.RowEstimate, detail.TotalBytes, detail.SizeHuman, err = e.fetchTableSize(ctx, detail.Schema, tableName)
	if err != nil {
		// Non-fatal: views and some system objects may not have size info.
		detail.RowEstimate = 0
		detail.TotalBytes = 0
		detail.SizeHuman = ""
	}

	detail.Columns, err = e.fetchColumns(ctx, detail.Schema, tableName)
	if err != nil {
		return nil, err
	}

	if err := e.markPrimaryKeys(ctx, detail); err != nil {
		return nil, err
	}

	// Enrich columns with pg_stats profiling data.
	if err := e.fetchColumnStats(ctx, detail.Schema, tableName, detail.Columns, detail.RowEstimate); err != nil {
		// Non-fatal: stats may not be available (e.g., never analyzed).
		// Columns are still returned without stats.
		_ = err
	}

	detail.ForeignKeys, err = e.fetchForeignKeys(ctx, detail.Schema, tableName)
	if err != nil {
		return nil, err
	}

	detail.Indexes, err = e.fetchIndexes(ctx, detail.Schema, tableName)
	if err != nil {
		return nil, err
	}

	detail.CheckConstraints, err = e.fetchCheckConstraints(ctx, detail.Schema, tableName)
	if err != nil {
		// Non-fatal: check constraints are enrichment, not essential.
		_ = err
	}

	// Fetch stats freshness.
	detail.StatsAge, err = e.fetchStatsAge(ctx, detail.Schema, tableName)
	if err != nil {
		_ = err
	}

	// Stats age warning.
	if detail.StatsAge != nil {
		age := time.Since(*detail.StatsAge)
		if age > 7*24*time.Hour {
			detail.StatsAgeWarning = fmt.Sprintf("Statistics are %.0f days old. Consider running ANALYZE on this table.", age.Hours()/24)
		}
	} else {
		detail.StatsAgeWarning = "No ANALYZE has been run on this table. Statistics may be missing or inaccurate."
	}

	// Sample rows (non-fatal).
	detail.SampleRows, err = fetchSampleRows(ctx, e.pool, detail.Schema, tableName)
	if err != nil {
		_ = err
	}

	// Index usage (non-fatal).
	detail.IndexUsage, err = fetchIndexUsage(ctx, e.pool, detail.Schema, tableName)
	if err != nil {
		_ = err
	}

	return detail, nil
}
