package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Profiler provides deep table profiling using Postgres system catalogs.
type Profiler struct {
	pool    *pgxpool.Pool
	schemas []string
}

func NewProfiler(pool *pgxpool.Pool, schemas []string) *Profiler {
	return &Profiler{pool: pool, schemas: schemas}
}

// schemaFilter returns a SQL WHERE clause fragment and args for filtering by schema.
func (p *Profiler) schemaFilter(column string, paramOffset int) (clause string, args []any) {
	if len(p.schemas) == 0 {
		return fmt.Sprintf("%s NOT IN ('pg_catalog', 'information_schema')", column), nil
	}
	placeholders := make([]string, len(p.schemas))
	args = make([]any, len(p.schemas))
	for i, s := range p.schemas {
		placeholders[i] = fmt.Sprintf("$%d", paramOffset+i)
		args[i] = s
	}
	return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", ")), args
}

func (p *Profiler) ProfileTable(ctx context.Context, schema, tableName string) (*port.TableProfile, error) {
	// Resolve schema if not provided.
	if schema == "" {
		var err error
		schema, err = p.resolveSchema(ctx, tableName)
		if err != nil {
			return nil, err
		}
	}

	profile := &port.TableProfile{
		Schema: schema,
		Name:   tableName,
	}

	// 1. Size breakdown.
	if err := p.fetchSizeBreakdown(ctx, schema, tableName, profile); err != nil {
		return nil, fmt.Errorf("profiling size: %w", err)
	}

	// 2. Sample rows.
	sampleRows, err := p.fetchSampleRows(ctx, schema, tableName)
	if err != nil {
		// Non-fatal: sampling may fail on views or empty tables.
		_ = err
	} else {
		profile.SampleRows = sampleRows
	}

	// 3. Index usage.
	profile.IndexUsage, err = p.fetchIndexUsage(ctx, schema, tableName)
	if err != nil {
		_ = err
	}

	// 4. Implicit FK candidates.
	profile.InferredFKs, err = p.inferForeignKeys(ctx, schema, tableName)
	if err != nil {
		_ = err
	}

	// 5. Stats freshness.
	profile.StatsAge, err = p.fetchStatsAge(ctx, schema, tableName)
	if err != nil {
		_ = err
	}
	if profile.StatsAge != nil {
		age := time.Since(*profile.StatsAge)
		if age > 7*24*time.Hour {
			profile.StatsAgeWarning = fmt.Sprintf("Statistics are %.0f days old. Consider running ANALYZE on this table.", age.Hours()/24)
		}
	} else {
		profile.StatsAgeWarning = "No ANALYZE has been run on this table. Statistics may be missing or inaccurate."
	}

	return profile, nil
}

func (p *Profiler) resolveSchema(ctx context.Context, tableName string) (string, error) {
	filter, filterArgs := p.schemaFilter("n.nspname", 2) // $1 is tableName
	query := fmt.Sprintf(`
		SELECT n.nspname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1 AND c.relkind IN ('r', 'p') AND %s
		LIMIT 1`, filter)

	args := make([]any, 0, 1+len(filterArgs))
	args = append(args, tableName)
	args = append(args, filterArgs...)

	var schema string
	err := p.pool.QueryRow(ctx, query, args...).Scan(&schema)
	if err != nil {
		return "", fmt.Errorf("table %q not found: %w", tableName, err)
	}
	return schema, nil
}

func (p *Profiler) fetchSizeBreakdown(ctx context.Context, schema, tableName string, profile *port.TableProfile) error {
	var toastBytes int64
	err := p.pool.QueryRow(ctx, queryTableSizeBreakdown, schema, tableName).
		Scan(&profile.RowEstimate, &profile.TotalBytes, &profile.TableBytes,
			&profile.IndexBytes, &toastBytes, &profile.SizeHuman)
	if err != nil {
		return err
	}
	if toastBytes > 0 {
		if profile.Extra == nil {
			profile.Extra = make(map[string]any)
		}
		profile.Extra["toast_bytes"] = toastBytes
	}
	return nil
}

func (p *Profiler) fetchSampleRows(ctx context.Context, schema, tableName string) ([]map[string]any, error) {
	// Use TABLESAMPLE BERNOULLI for sampling — it works at the row level (not page level
	// like SYSTEM), so it returns rows even on small tables. Use a generous 50% sample
	// rate with LIMIT 5 to get a handful of representative rows.
	fqn := fmt.Sprintf("%s.%s", quoteIdent(schema), quoteIdent(tableName))
	query := fmt.Sprintf("SELECT * FROM %s TABLESAMPLE BERNOULLI(50) LIMIT 5", fqn)

	rows, err := p.pool.Query(ctx, query)
	if err != nil {
		// Fallback: TABLESAMPLE may not work on some table types (e.g., foreign tables).
		query = fmt.Sprintf("SELECT * FROM %s LIMIT 5", fqn)
		rows, err = p.pool.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("sampling rows: %w", err)
		}
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	var result []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scanning sample row: %w", err)
		}
		row := make(map[string]any, len(fields))
		for i, fd := range fields {
			row[fd.Name] = vals[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (p *Profiler) fetchIndexUsage(ctx context.Context, schema, tableName string) ([]port.IndexUsage, error) {
	rows, err := p.pool.Query(ctx, queryIndexUsage, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("querying index usage: %w", err)
	}
	defer rows.Close()

	var usage []port.IndexUsage
	for rows.Next() {
		var u port.IndexUsage
		if err := rows.Scan(&u.Name, &u.Scans, &u.SizeBytes, &u.SizeHuman); err != nil {
			return nil, fmt.Errorf("scanning index usage: %w", err)
		}
		usage = append(usage, u)
	}
	return usage, rows.Err()
}

func (p *Profiler) fetchStatsAge(ctx context.Context, schema, tableName string) (*time.Time, error) {
	var ts *time.Time
	err := p.pool.QueryRow(ctx, queryStatsAge, schema, tableName).Scan(&ts)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return ts, nil
}

// inferForeignKeys detects implicit FK relationships by matching *_id column naming patterns
// against primary key columns in other tables.
func (p *Profiler) inferForeignKeys(ctx context.Context, schema, tableName string) ([]port.InferredFK, error) {
	// First, get columns of the target table.
	targetCols, err := p.getTableColumns(ctx, schema, tableName)
	if err != nil {
		return nil, err
	}

	// Get explicit FKs to exclude them.
	explicitFKs, err := p.getExplicitFKColumns(ctx, schema, tableName)
	if err != nil {
		return nil, err
	}

	// Get all tables with their PK columns for matching.
	pkIndex, err := p.buildPKIndex(ctx)
	if err != nil {
		return nil, err
	}

	// Build table name set for the domain matching function.
	tableNames := make(map[string]bool, len(pkIndex))
	for tbl := range pkIndex {
		tableNames[tbl] = true
	}

	var inferred []port.InferredFK
	for _, col := range targetCols {
		// Skip columns that already have explicit FKs.
		if explicitFKs[col.name] {
			continue
		}

		// Use domain naming pattern match, then verify type compatibility (adapter-specific).
		candidate, ok := domain.MatchFKNamingPattern(col.name, tableNames)
		if !ok {
			continue
		}

		pk, pkOK := pkIndex[candidate.ReferencedTable]
		if !pkOK {
			continue
		}

		if isTypeCompatible(col.dataType, pk.dataType) {
			inferred = append(inferred, port.InferredFK{
				ColumnName:       candidate.ColumnName,
				ReferencedTable:  candidate.ReferencedTable,
				ReferencedColumn: pk.colName,
				Confidence:       candidate.Confidence,
				Reason:           candidate.Reason,
			})
		}
	}
	return inferred, nil
}

type colInfo struct {
	name     string
	dataType string
}

type pkInfo struct {
	colName  string
	dataType string
}

func (p *Profiler) getTableColumns(ctx context.Context, schema, tableName string) ([]colInfo, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("getting table columns: %w", err)
	}
	defer rows.Close()

	var cols []colInfo
	for rows.Next() {
		var c colInfo
		if err := rows.Scan(&c.name, &c.dataType); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func (p *Profiler) getExplicitFKColumns(ctx context.Context, schema, tableName string) (map[string]bool, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2`, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("getting explicit FKs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		result[col] = true
	}
	return result, rows.Err()
}

func (p *Profiler) buildPKIndex(ctx context.Context) (map[string]pkInfo, error) {
	filter, args := p.schemaFilter("n.nspname", 1)
	query := fmt.Sprintf(`
		SELECT c.relname, a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod)
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indisprimary AND %s`, filter)

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("building PK index: %w", err)
	}
	defer rows.Close()

	// Map table name → PK info. For composite PKs, take the first column only.
	result := make(map[string]pkInfo)
	for rows.Next() {
		var tableName, colName, dataType string
		if err := rows.Scan(&tableName, &colName, &dataType); err != nil {
			return nil, err
		}
		if _, exists := result[tableName]; !exists {
			result[tableName] = pkInfo{colName: colName, dataType: dataType}
		}
	}
	return result, rows.Err()
}

// isTypeCompatible checks if two column types are compatible for FK inference.
func isTypeCompatible(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	intTypes := map[string]bool{
		"integer": true, "bigint": true, "smallint": true, "int": true,
		"int4": true, "int8": true, "int2": true, "serial": true,
		"bigserial": true, "smallserial": true,
	}

	uuidTypes := map[string]bool{"uuid": true}
	textTypes := map[string]bool{"text": true, "character varying": true, "varchar": true}

	if intTypes[a] && intTypes[b] {
		return true
	}
	if uuidTypes[a] && uuidTypes[b] {
		return true
	}
	if textTypes[a] && textTypes[b] {
		return true
	}
	return a == b
}

// quoteIdent quotes a SQL identifier to prevent injection.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
