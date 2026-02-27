package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchFKNamingPattern(t *testing.T) {
	t.Parallel()
	tables := map[string]bool{
		"users":      true,
		"products":   true,
		"categories": true,
		"status":     true, // singular table name
	}

	tests := []struct {
		name      string
		column    string
		wantMatch bool
		wantTable string
		wantConf  string
	}{
		{"plural match", "user_id", true, "users", "high"},
		{"plural match 2", "product_id", true, "products", "high"},
		{"singular match", "status_id", true, "status", "high"},
		{"es-suffix match", "categori_id", true, "categories", "medium"},
		{"no _id suffix", "username", false, "", ""},
		{"no matching table", "order_id", false, "", ""},
		{"just _id", "_id", false, "", ""},
		{"exact prefix no table", "widget_id", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			candidate, ok := MatchFKNamingPattern(tt.column, tables)
			assert.Equal(t, tt.wantMatch, ok)
			if ok {
				assert.Equal(t, tt.wantTable, candidate.ReferencedTable)
				assert.Equal(t, tt.wantConf, candidate.Confidence)
				assert.Equal(t, tt.column, candidate.ColumnName)
				assert.Equal(t, "id", candidate.ReferencedPK)
				assert.NotEmpty(t, candidate.Reason)
			}
		})
	}
}
