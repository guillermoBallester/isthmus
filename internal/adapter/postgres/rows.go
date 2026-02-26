package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5"
)

// rowsToMaps converts pgx.Rows into a slice of maps keyed by column name.
func rowsToMaps(rows pgx.Rows) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	var result []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("reading row values: %w", err)
		}
		row := make(map[string]any, len(fields))
		for i, fd := range fields {
			row[fd.Name] = vals[i]
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}
	return result, nil
}
