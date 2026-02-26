package domain

import (
	"fmt"
	"strings"
)

// FKCandidate represents a possible foreign key relationship inferred from
// column naming patterns. Type compatibility checking is left to the adapter
// because type names are database-specific.
type FKCandidate struct {
	ColumnName      string // e.g. "user_id"
	ReferencedTable string // e.g. "users"
	ReferencedPK    string // e.g. "id" (assumed PK column name)
	Confidence      string // "high" or "medium"
	Reason          string
}

// MatchFKNamingPattern checks whether columnName follows the *_id naming
// convention and matches a known table name (plural or singular form).
// tableNames is the set of all table names in scope. Returns a candidate
// and true if a match is found, or zero value and false otherwise.
func MatchFKNamingPattern(columnName string, tableNames map[string]bool) (FKCandidate, bool) {
	if !strings.HasSuffix(columnName, "_id") {
		return FKCandidate{}, false
	}
	prefix := strings.TrimSuffix(columnName, "_id")

	// Try plural and singular forms of the table name.
	candidates := []string{prefix + "s", prefix, prefix + "es"}
	for _, candidate := range candidates {
		if !tableNames[candidate] {
			continue
		}
		confidence := "high"
		if candidate != prefix+"s" && candidate != prefix {
			confidence = "medium"
		}
		return FKCandidate{
			ColumnName:      columnName,
			ReferencedTable: candidate,
			ReferencedPK:    "id",
			Confidence:      confidence,
			Reason:          fmt.Sprintf("column %q matches naming pattern for table %q", columnName, candidate),
		}, true
	}
	return FKCandidate{}, false
}
