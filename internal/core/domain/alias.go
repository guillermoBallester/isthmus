package domain

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ExtractAliasMap parses a SQL SELECT statement and returns a map of
// original column name → alias for every column that uses an AS clause.
// Only simple column references are considered (e.g. "Email" AS email,
// c."Email" AS email). Expressions are skipped because they won't match
// any mask key. Returns an empty map on parse error (fail-open to
// preserve current behavior).
func ExtractAliasMap(sql string) map[string]string {
	aliases := make(map[string]string)

	tree, err := pg_query.Parse(sql)
	if err != nil || len(tree.Stmts) == 0 {
		return aliases
	}

	stmt := tree.Stmts[0].Stmt
	if stmt == nil {
		return aliases
	}

	sel, ok := stmt.Node.(*pg_query.Node_SelectStmt)
	if !ok {
		return aliases
	}

	for _, target := range sel.SelectStmt.TargetList {
		rt, ok := target.Node.(*pg_query.Node_ResTarget)
		if !ok || rt.ResTarget == nil {
			continue
		}

		alias := rt.ResTarget.Name
		if alias == "" {
			continue // no alias on this target
		}

		val := rt.ResTarget.Val
		if val == nil {
			continue
		}

		cr, ok := val.Node.(*pg_query.Node_ColumnRef)
		if !ok || cr.ColumnRef == nil {
			continue // not a simple column reference (expression, etc.)
		}

		// Extract the bare column name from the last field of the ColumnRef.
		// For "Email" → Fields = [String{Sval:"Email"}]
		// For c."Email" → Fields = [String{Sval:"c"}, String{Sval:"Email"}]
		fields := cr.ColumnRef.Fields
		if len(fields) == 0 {
			continue
		}

		lastField := fields[len(fields)-1]
		str, ok := lastField.Node.(*pg_query.Node_String_)
		if !ok || str.String_ == nil {
			continue
		}

		colName := str.String_.Sval
		if colName != "" && colName != alias {
			aliases[colName] = alias
		}
	}

	return aliases
}
