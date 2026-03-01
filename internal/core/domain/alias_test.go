package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAliasMap_SimpleAlias(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap(`SELECT "Email" AS email FROM "Customer"`)
	assert.Equal(t, map[string]string{"Email": "email"}, aliases)
}

func TestExtractAliasMap_TableQualified(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap(`SELECT c."Email" AS email FROM "Customer" c`)
	assert.Equal(t, map[string]string{"Email": "email"}, aliases)
}

func TestExtractAliasMap_NoAlias(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap(`SELECT "Email" FROM "Customer"`)
	assert.Empty(t, aliases)
}

func TestExtractAliasMap_Mixed(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap(`SELECT "CustomerId", "Email" AS email, "Phone" AS phone FROM "Customer"`)
	assert.Equal(t, map[string]string{
		"Email": "email",
		"Phone": "phone",
	}, aliases)
}

func TestExtractAliasMap_Expression(t *testing.T) {
	t.Parallel()
	// Expressions (concatenation) are not simple ColumnRefs — should be skipped.
	aliases := ExtractAliasMap(`SELECT "FirstName" || ' ' || "LastName" AS name FROM "Customer"`)
	assert.Empty(t, aliases)
}

func TestExtractAliasMap_CasePreservation(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap(`SELECT "FirstName" AS first_name FROM "Customer"`)
	assert.Equal(t, map[string]string{"FirstName": "first_name"}, aliases)
}

func TestExtractAliasMap_SameNameAlias(t *testing.T) {
	t.Parallel()
	// If alias matches column name, no entry (no-op).
	aliases := ExtractAliasMap(`SELECT "Email" AS "Email" FROM "Customer"`)
	assert.Empty(t, aliases)
}

func TestExtractAliasMap_InvalidSQL(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap("NOT VALID SQL !!!")
	assert.Empty(t, aliases)
}

func TestExtractAliasMap_EmptySQL(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap("")
	assert.Empty(t, aliases)
}

func TestExtractAliasMap_NonSelect(t *testing.T) {
	t.Parallel()
	aliases := ExtractAliasMap("INSERT INTO foo (a) VALUES (1)")
	assert.Empty(t, aliases)
}

func TestExtractAliasMap_MultipleTableQualified(t *testing.T) {
	t.Parallel()
	sql := `SELECT c."FirstName" AS first_name, c."LastName" AS last_name, c."Email" FROM "Customer" c`
	aliases := ExtractAliasMap(sql)
	assert.Equal(t, map[string]string{
		"FirstName": "first_name",
		"LastName":  "last_name",
	}, aliases)
}
