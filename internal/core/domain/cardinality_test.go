package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyByDistinctCount(t *testing.T) {
	tests := []struct {
		name          string
		distinctCount int64
		totalRows     int64
		want          CardinalityClass
	}{
		{"all unique", 1000, 1000, CardinalityUnique},
		{"near unique (95%)", 950, 1000, CardinalityNearUnique},
		{"near unique threshold (90%)", 900, 1000, CardinalityNearUnique},
		{"high cardinality (50%)", 500, 1000, CardinalityHighCardinality},
		{"enum-like (5 distinct)", 5, 1000, CardinalityEnumLike},
		{"enum-like (20 distinct)", 20, 1000, CardinalityEnumLike},
		{"low cardinality (50 distinct)", 50, 1000, CardinalityLowCardinality},
		{"low cardinality (200 distinct)", 200, 1000, CardinalityLowCardinality},
		{"high cardinality (500 distinct)", 500, 1000, CardinalityHighCardinality},
		{"zero distinct", 0, 1000, CardinalityEnumLike},
		{"one distinct", 1, 1000, CardinalityEnumLike},
		{"zero rows zero distinct", 0, 0, CardinalityEnumLike},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyByDistinctCount(tt.distinctCount, tt.totalRows)
			assert.Equal(t, tt.want, got)
		})
	}
}
