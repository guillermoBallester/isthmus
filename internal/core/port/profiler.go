package port

import (
	"context"
	"time"
)

// IndexUsage holds usage statistics for a single index.
type IndexUsage struct {
	Name      string `json:"name"`
	Scans     int64  `json:"scans"`
	SizeBytes int64  `json:"size_bytes"`
	SizeHuman string `json:"size_human"`
}

// InferredFK represents a foreign key relationship inferred from naming patterns.
type InferredFK struct {
	ColumnName       string `json:"column_name"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
	Confidence       string `json:"confidence"` // "high", "medium", "low"
	Reason           string `json:"reason"`
}

// TableProfile holds deep analysis data for a single table.
type TableProfile struct {
	Schema          string           `json:"schema"`
	Name            string           `json:"name"`
	RowEstimate     int64            `json:"row_estimate"`
	TotalBytes      int64            `json:"total_bytes"`
	TableBytes      int64            `json:"table_bytes"`
	IndexBytes      int64            `json:"index_bytes"`
	SizeHuman       string           `json:"size_human"`
	SampleRows      []map[string]any `json:"sample_rows,omitempty"`
	IndexUsage      []IndexUsage     `json:"index_usage,omitempty"`
	InferredFKs     []InferredFK     `json:"inferred_fks,omitempty"`
	StatsAge        *time.Time       `json:"stats_age,omitempty"`
	StatsAgeWarning string           `json:"stats_age_warning,omitempty"`
	Extra           map[string]any   `json:"extra,omitempty"`
}

// SchemaProfiler provides deep table profiling beyond basic schema exploration.
type SchemaProfiler interface {
	ProfileTable(ctx context.Context, schema, tableName string) (*TableProfile, error)
}
