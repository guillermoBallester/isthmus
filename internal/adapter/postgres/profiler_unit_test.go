package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPgDistinctToAbsolute(t *testing.T) {
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
			got := pgDistinctToAbsolute(tt.nDistinct, tt.rowEstimate)
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
