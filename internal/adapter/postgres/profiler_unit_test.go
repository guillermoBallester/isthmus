package postgres

import (
	"testing"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/stretchr/testify/assert"
)

func TestClassifyCardinality(t *testing.T) {
	tests := []struct {
		name        string
		nDistinct   float64
		rowEstimate int64
		want        port.CardinalityClass
	}{
		{"all unique (-1)", -1, 1000, port.CardinalityUnique},
		{"near unique (-0.95)", -0.95, 1000, port.CardinalityNearUnique},
		{"near unique threshold (-0.9)", -0.9, 1000, port.CardinalityNearUnique},
		{"high cardinality (-0.5)", -0.5, 1000, port.CardinalityHighCardinality},
		{"enum-like (5 distinct)", 5, 1000, port.CardinalityEnumLike},
		{"enum-like (20 distinct)", 20, 1000, port.CardinalityEnumLike},
		{"low cardinality (50 distinct)", 50, 1000, port.CardinalityLowCardinality},
		{"low cardinality (200 distinct)", 200, 1000, port.CardinalityLowCardinality},
		{"high cardinality (500 distinct)", 500, 1000, port.CardinalityHighCardinality},
		{"enum-like from ratio (-0.005 with 1000 rows = 5 distinct)", -0.005, 1000, port.CardinalityEnumLike},
		{"low cardinality from ratio (-0.1 with 1000 rows = 100 distinct)", -0.1, 1000, port.CardinalityLowCardinality},
		{"zero distinct", 0, 1000, port.CardinalityEnumLike},
		{"one distinct", 1, 1000, port.CardinalityEnumLike},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCardinality(tt.nDistinct, tt.rowEstimate)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePgArray(t *testing.T) {
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
			got := parsePgArray(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePgFloatArray(t *testing.T) {
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
			got := parsePgFloatArray(tt.input)
			assert.InDeltaSlice(t, tt.want, got, 0.001)
		})
	}
}

func TestIsTypeCompatible(t *testing.T) {
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
			assert.Equal(t, tt.want, isTypeCompatible(tt.a, tt.b))
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	assert.Equal(t, `"users"`, quoteIdent("users"))
	assert.Equal(t, `"my table"`, quoteIdent("my table"))
	assert.Equal(t, `"test""quote"`, quoteIdent(`test"quote`))
}
