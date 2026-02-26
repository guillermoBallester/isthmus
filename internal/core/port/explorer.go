package port

import (
	"context"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
)

// ColumnStats holds profiling data for a single column.
type ColumnStats struct {
	NullFraction    float64                 `json:"null_fraction"`
	Cardinality     domain.CardinalityClass `json:"cardinality"`
	DistinctCount   int64                   `json:"distinct_count"`
	MostCommonVals  []string                `json:"most_common_vals,omitempty"`
	MostCommonFreqs []float64               `json:"most_common_freqs,omitempty"`
	MinValue        string                  `json:"min_value,omitempty"`
	MaxValue        string                  `json:"max_value,omitempty"`
}

type TableInfo struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	RowEstimate int64  `json:"row_estimate"`
	TotalBytes  int64  `json:"total_bytes,omitempty"`
	SizeHuman   string `json:"size_human,omitempty"`
	ColumnCount int    `json:"column_count"`
	HasIndexes  bool   `json:"has_indexes"`
	Comment     string `json:"comment,omitempty"`
}

type ColumnInfo struct {
	Name         string       `json:"name"`
	DataType     string       `json:"data_type"`
	IsNullable   bool         `json:"is_nullable"`
	DefaultValue string       `json:"default_value,omitempty"`
	IsPrimaryKey bool         `json:"is_primary_key"`
	Comment      string       `json:"comment,omitempty"`
	Stats        *ColumnStats `json:"stats,omitempty"`
}

type ForeignKey struct {
	ConstraintName   string `json:"constraint_name"`
	ColumnName       string `json:"column_name"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
}

type CheckConstraint struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

type IndexInfo struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	IsUnique   bool   `json:"is_unique"`
}

type TableDetail struct {
	Schema           string            `json:"schema"`
	Name             string            `json:"name"`
	Comment          string            `json:"comment,omitempty"`
	RowEstimate      int64             `json:"row_estimate"`
	TotalBytes       int64             `json:"total_bytes,omitempty"`
	SizeHuman        string            `json:"size_human,omitempty"`
	Columns          []ColumnInfo      `json:"columns"`
	ForeignKeys      []ForeignKey      `json:"foreign_keys,omitempty"`
	Indexes          []IndexInfo       `json:"indexes,omitempty"`
	CheckConstraints []CheckConstraint `json:"check_constraints,omitempty"`
	StatsAge         *time.Time        `json:"stats_age,omitempty"`
}

type SchemaInfo struct {
	Name string `json:"name"`
}

type SchemaExplorer interface {
	ListSchemas(ctx context.Context) ([]SchemaInfo, error)
	ListTables(ctx context.Context) ([]TableInfo, error)
	DescribeTable(ctx context.Context, schema, tableName string) (*TableDetail, error)
}
