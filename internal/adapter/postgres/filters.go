package postgres

import (
	"fmt"
	"strings"
)

// schemaFilter returns a SQL WHERE clause fragment and args for filtering by schema.
// paramOffset is the starting $N parameter index (1-based).
// When schemas is empty, it excludes system schemas (pg_catalog, information_schema).
func schemaFilter(schemas []string, column string, paramOffset int) (clause string, args []any) {
	if len(schemas) == 0 {
		return fmt.Sprintf("%s NOT IN ('pg_catalog', 'information_schema')", column), nil
	}
	placeholders := make([]string, len(schemas))
	args = make([]any, len(schemas))
	for i, s := range schemas {
		placeholders[i] = fmt.Sprintf("$%d", paramOffset+i)
		args[i] = s
	}
	return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", ")), args
}

// quoteIdent quotes a SQL identifier to prevent injection.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
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
