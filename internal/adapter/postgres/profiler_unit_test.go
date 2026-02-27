package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgDistinctToAbsolute(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		nDistinct   float64
		rowEstimate int64
		want        int64
	}{
		{"all unique (-1)", -1, 1000, 1000},
		{"fraction (-0.5)", -0.5, 1000, 500},
		{"fraction (-0.95)", -0.95, 1000, 950},
		{"fraction (-0.005)", -0.005, 1000, 5},
		{"positive integer", 42, 1000, 42},
		{"positive float rounds", 42.6, 1000, 43},
		{"zero", 0, 1000, 0},
		{"one", 1, 1000, 1},
		{"zero rows unique", -1, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pgDistinctToAbsolute(tt.nDistinct, tt.rowEstimate)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePgArray(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "{}", nil},
		{"blank", "", nil},
		{"single", "{active}", []string{"active"}},
		{"multiple", "{active,inactive,pending}", []string{"active", "inactive", "pending"}},
		{"with spaces", "{ active , inactive , pending }", []string{"active", "inactive", "pending"}},
		{"quoted", `{"hello world","foo bar"}`, []string{"hello world", "foo bar"}},
		{"with NULL", "{active,NULL,pending}", []string{"active", "pending"}},
		{"numeric", "{1,2,3,4,5}", []string{"1", "2", "3", "4", "5"}},
		{"escaped quotes", `{"he said \"hi\"",normal}`, []string{`he said "hi"`, "normal"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parsePgArray(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePgFloatArray(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []float64
	}{
		{"empty", "{}", nil},
		{"single", "{0.5}", []float64{0.5}},
		{"multiple", "{0.5,0.3,0.2}", []float64{0.5, 0.3, 0.2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parsePgFloatArray(tt.input)
			assert.InDeltaSlice(t, tt.want, got, 0.001)
		})
	}
}

func TestIsTypeCompatible(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want bool
	}{
		{"integer", "bigint", true},
		{"integer", "integer", true},
		{"int4", "int8", true},
		{"uuid", "uuid", true},
		{"text", "character varying", true},
		{"integer", "text", false},
		{"uuid", "integer", false},
		{"boolean", "boolean", true},
		{"boolean", "integer", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isTypeCompatible(tt.a, tt.b))
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, `"users"`, quoteIdent("users"))
	assert.Equal(t, `"my table"`, quoteIdent("my table"))
	assert.Equal(t, `"test""quote"`, quoteIdent(`test"quote`))
	assert.Equal(t, `""`, quoteIdent(""))
	assert.Equal(t, `"café"`, quoteIdent("café"))
	assert.Equal(t, `""""""""`, quoteIdent(`"""`))
}

func TestSchemaFilter_Empty(t *testing.T) {
	t.Parallel()
	clause, args := schemaFilter(nil, "n.nspname", 1)
	assert.Equal(t, "n.nspname NOT IN ('pg_catalog', 'information_schema')", clause)
	assert.Nil(t, args)
}

func TestSchemaFilter_Single(t *testing.T) {
	t.Parallel()
	clause, args := schemaFilter([]string{"public"}, "n.nspname", 1)
	assert.Equal(t, "n.nspname IN ($1)", clause)
	assert.Equal(t, []any{"public"}, args)
}

func TestSchemaFilter_Multiple(t *testing.T) {
	t.Parallel()
	clause, args := schemaFilter([]string{"public", "app", "sales"}, "s.schema_name", 1)
	assert.Equal(t, "s.schema_name IN ($1, $2, $3)", clause)
	assert.Equal(t, []any{"public", "app", "sales"}, args)
}

func TestSchemaFilter_ParamOffset(t *testing.T) {
	t.Parallel()
	clause, args := schemaFilter([]string{"public", "app"}, "t.table_schema", 3)
	assert.Equal(t, "t.table_schema IN ($3, $4)", clause)
	assert.Equal(t, []any{"public", "app"}, args)
}

func TestParsePgArray_EscapedBrace(t *testing.T) {
	t.Parallel()
	// Outer } is stripped by TrimSuffix, so the escaped char is consumed by brace stripping.
	// Input after brace stripping: hello\  → escape consumes nothing useful → "hello".
	got := parsePgArray(`{hello\}`)
	assert.Equal(t, []string{"hello"}, got)
}

func TestParsePgArray_ConsecutiveCommas(t *testing.T) {
	t.Parallel()
	// Consecutive commas produce empty strings (not NULL-filtered).
	got := parsePgArray("{a,,b}")
	assert.Equal(t, []string{"a", "", "b"}, got)
}

func TestParsePgArray_AllNULL(t *testing.T) {
	t.Parallel()
	got := parsePgArray("{NULL,NULL}")
	assert.Nil(t, got)
}

func TestParsePgFloatArray_InvalidValues(t *testing.T) {
	got := parsePgFloatArray("{0.5,notanumber,0.3}")
	assert.InDeltaSlice(t, []float64{0.5, 0.3}, got, 0.001)
}

func TestParsePgFloatArray_WithNULL(t *testing.T) {
	got := parsePgFloatArray("{0.1,NULL,0.9}")
	assert.InDeltaSlice(t, []float64{0.1, 0.9}, got, 0.001)
}

func TestParsePgFloatArray_LargeValues(t *testing.T) {
	got := parsePgFloatArray("{1e10,0.0000001}")
	require.Len(t, got, 2)
	assert.InDelta(t, 1e10, got[0], 1)
	assert.InDelta(t, 0.0000001, got[1], 1e-10)
}

func TestIsTypeCompatible_MixedCase(t *testing.T) {
	assert.True(t, isTypeCompatible("INTEGER", "bigint"))
	assert.True(t, isTypeCompatible("Uuid", "UUID"))
	assert.True(t, isTypeCompatible("TEXT", "varchar"))
}

func TestIsTypeCompatible_SerialTypes(t *testing.T) {
	assert.True(t, isTypeCompatible("serial", "integer"))
	assert.True(t, isTypeCompatible("bigserial", "bigint"))
	assert.True(t, isTypeCompatible("smallserial", "smallint"))
}

func TestIsTypeCompatible_VarcharCompat(t *testing.T) {
	assert.True(t, isTypeCompatible("varchar", "text"))
	assert.True(t, isTypeCompatible("character varying", "varchar"))
}

func TestIsTypeCompatible_UnknownTypes(t *testing.T) {
	assert.True(t, isTypeCompatible("jsonb", "jsonb"))
	assert.False(t, isTypeCompatible("jsonb", "json"))
}
