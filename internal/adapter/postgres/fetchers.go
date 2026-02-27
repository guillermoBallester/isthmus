package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/jackc/pgx/v5"
)

func (e *Explorer) fetchTableComment(ctx context.Context, schema, tableName string) (string, error) {
	var comment string
	err := e.pool.QueryRow(ctx, queryTableComment, schema, tableName).Scan(&comment)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("table %q %w in schema %q", tableName, domain.ErrNotFound, schema)
		}
		return "", fmt.Errorf("table %q not found in schema %q: %w", tableName, schema, err)
	}
	return comment, nil
}

func (e *Explorer) fetchTableMeta(ctx context.Context, tableName string) (schema, comment string, err error) {
	filter, filterArgs := schemaFilter(e.schemas, "t.table_schema", 2) // $1 is tableName
	query := fmt.Sprintf(queryTableMeta, filter)

	args := make([]any, 0, 1+len(filterArgs))
	args = append(args, tableName)
	args = append(args, filterArgs...)

	err = e.pool.QueryRow(ctx, query, args...).Scan(&schema, &comment)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if len(e.schemas) > 0 {
				return "", "", fmt.Errorf("table %q %w in schemas %v", tableName, domain.ErrNotFound, e.schemas)
			}
			return "", "", fmt.Errorf("table %q %w", tableName, domain.ErrNotFound)
		}
		return "", "", fmt.Errorf("querying table metadata for %q: %w", tableName, err)
	}
	return schema, comment, nil
}

func (e *Explorer) fetchColumns(ctx context.Context, schema, tableName string) ([]port.ColumnInfo, error) {
	rows, err := e.pool.Query(ctx, queryColumns, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	var cols []port.ColumnInfo
	for rows.Next() {
		var col port.ColumnInfo
		if err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.DefaultValue, &col.Comment); err != nil {
			return nil, fmt.Errorf("scanning column: %w", err)
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func (e *Explorer) markPrimaryKeys(ctx context.Context, detail *port.TableDetail) error {
	rows, err := e.pool.Query(ctx, queryPrimaryKeys, detail.Schema, detail.Name)
	if err != nil {
		return fmt.Errorf("querying primary keys: %w", err)
	}
	defer rows.Close()

	pkCols := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scanning pk: %w", err)
		}
		pkCols[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range detail.Columns {
		if pkCols[detail.Columns[i].Name] {
			detail.Columns[i].IsPrimaryKey = true
		}
	}
	return nil
}

func (e *Explorer) fetchForeignKeys(ctx context.Context, schema, tableName string) ([]port.ForeignKey, error) {
	rows, err := e.pool.Query(ctx, queryForeignKeys, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []port.ForeignKey
	for rows.Next() {
		var fk port.ForeignKey
		if err := rows.Scan(&fk.ConstraintName, &fk.ColumnName, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, fmt.Errorf("scanning fk: %w", err)
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (e *Explorer) fetchIndexes(ctx context.Context, schema, tableName string) ([]port.IndexInfo, error) {
	rows, err := e.pool.Query(ctx, queryIndexes, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("querying indexes: %w", err)
	}
	defer rows.Close()

	var idxs []port.IndexInfo
	for rows.Next() {
		var idx port.IndexInfo
		if err := rows.Scan(&idx.Name, &idx.Definition, &idx.IsUnique); err != nil {
			return nil, fmt.Errorf("scanning index: %w", err)
		}
		idxs = append(idxs, idx)
	}
	return idxs, rows.Err()
}

// fetchColumnStats reads pg_stats for all columns in a table and enriches the
// column list with cardinality classification, null fraction, common values, and ranges.
func (e *Explorer) fetchColumnStats(ctx context.Context, schema, tableName string, columns []port.ColumnInfo, rowEstimate int64) error {
	rows, err := e.pool.Query(ctx, queryColumnStats, schema, tableName)
	if err != nil {
		return fmt.Errorf("querying column stats: %w", err)
	}
	defer rows.Close()

	statsMap := make(map[string]*port.ColumnStats)
	for rows.Next() {
		var (
			attname      string
			nullFrac     float64
			nDistinct    float64
			mcvRaw       *string
			mcfRaw       *string
			histogramRaw *string
		)
		if err := rows.Scan(&attname, &nullFrac, &nDistinct, &mcvRaw, &mcfRaw, &histogramRaw); err != nil {
			return fmt.Errorf("scanning column stats: %w", err)
		}

		absDistinct := pgDistinctToAbsolute(nDistinct, rowEstimate)
		stats := &port.ColumnStats{
			NullFraction:  nullFrac,
			DistinctCount: absDistinct,
			Cardinality:   domain.ClassifyByDistinctCount(absDistinct, rowEstimate),
		}

		// Parse most common values for enum-like columns.
		if stats.Cardinality == domain.CardinalityEnumLike && mcvRaw != nil {
			stats.MostCommonVals = parsePgArray(*mcvRaw)
			if mcfRaw != nil {
				stats.MostCommonFreqs = parsePgFloatArray(*mcfRaw)
			}
		}

		// Parse histogram bounds for min/max range.
		if histogramRaw != nil {
			bounds := parsePgArray(*histogramRaw)
			if len(bounds) >= 2 {
				stats.MinValue = bounds[0]
				stats.MaxValue = bounds[len(bounds)-1]
			}
		}

		statsMap[attname] = stats
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating column stats: %w", err)
	}

	for i := range columns {
		if s, ok := statsMap[columns[i].Name]; ok {
			columns[i].Stats = s
		}
	}
	return nil
}

// fetchCheckConstraints reads CHECK constraints for a table.
func (e *Explorer) fetchCheckConstraints(ctx context.Context, schema, tableName string) ([]port.CheckConstraint, error) {
	rows, err := e.pool.Query(ctx, queryCheckConstraints, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("querying check constraints: %w", err)
	}
	defer rows.Close()

	var checks []port.CheckConstraint
	for rows.Next() {
		var ck port.CheckConstraint
		if err := rows.Scan(&ck.Name, &ck.Expression); err != nil {
			return nil, fmt.Errorf("scanning check constraint: %w", err)
		}
		checks = append(checks, ck)
	}
	return checks, rows.Err()
}

// fetchTableSize reads row estimate, total size in bytes, and human-readable size from pg_class.
func (e *Explorer) fetchTableSize(ctx context.Context, schema, tableName string) (rowEstimate, totalBytes int64, sizeHuman string, err error) {
	err = e.pool.QueryRow(ctx, queryTableSize, schema, tableName).
		Scan(&rowEstimate, &totalBytes, &sizeHuman)
	if err != nil {
		return 0, 0, "", fmt.Errorf("querying table size: %w", err)
	}
	return rowEstimate, totalBytes, sizeHuman, nil
}

// fetchStatsAge reads the last ANALYZE timestamp for a table.
func (e *Explorer) fetchStatsAge(ctx context.Context, schema, tableName string) (*time.Time, error) {
	var ts *time.Time
	err := e.pool.QueryRow(ctx, queryStatsAge, schema, tableName).Scan(&ts)
	if err != nil {
		// No stats is not an error — could be a fresh table.
		return nil, nil //nolint:nilerr
	}
	return ts, nil
}

// pgDistinctToAbsolute converts pg_stats n_distinct to an absolute distinct count.
// pg_stats semantics:
//   - -1.0 = all values unique → returns rowEstimate
//   - negative = fraction of rows that are distinct (e.g., -0.5 = 50% unique)
//   - positive = estimated number of distinct values
func pgDistinctToAbsolute(nDistinct float64, rowEstimate int64) int64 {
	if nDistinct == -1 {
		return rowEstimate
	}
	if nDistinct < 0 {
		return int64(math.Round(-nDistinct * float64(rowEstimate)))
	}
	return int64(math.Round(nDistinct))
}

// parsePgArray parses a PostgreSQL text array representation like {val1,val2,val3}.
// Handles basic quoting but not all edge cases (sufficient for display purposes).
func parsePgArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil
	}
	// Strip outer braces.
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")

	var result []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for _, ch := range raw {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		switch {
		case ch == '\\':
			escaped = true
		case ch == '"':
			inQuote = !inQuote
		case ch == ',' && !inQuote:
			val := strings.TrimSpace(current.String())
			if val != "NULL" {
				result = append(result, val)
			}
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	// Flush last value.
	if current.Len() > 0 {
		val := strings.TrimSpace(current.String())
		if val != "NULL" {
			result = append(result, val)
		}
	}
	return result
}

// parsePgFloatArray parses a PostgreSQL float array like {0.5,0.3,0.2}.
func parsePgFloatArray(raw string) []float64 {
	vals := parsePgArray(raw)
	result := make([]float64, 0, len(vals))
	for _, v := range vals {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			result = append(result, f)
		}
	}
	return result
}
