package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyMask_Redact(t *testing.T) {
	assert.Equal(t, "***", ApplyMask("secret@email.com", "redact"))
	assert.Equal(t, "***", ApplyMask(12345, "redact"))
	assert.Nil(t, ApplyMask(nil, "redact"))
}

func TestApplyMask_Hash(t *testing.T) {
	result := ApplyMask("secret@email.com", "hash")
	s, ok := result.(string)
	assert.True(t, ok)
	assert.Len(t, s, 16, "hash should be 16 hex chars")

	// Deterministic: same input → same hash.
	assert.Equal(t, result, ApplyMask("secret@email.com", "hash"))

	// Different input → different hash.
	assert.NotEqual(t, result, ApplyMask("other@email.com", "hash"))

	assert.Nil(t, ApplyMask(nil, "hash"))
}

func TestApplyMask_Partial(t *testing.T) {
	assert.Equal(t, "******7890", ApplyMask("1234567890", "partial"))
	assert.Equal(t, "***ab", ApplyMask("ab", "partial"))
	assert.Equal(t, "***abcd", ApplyMask("abcd", "partial"))
	assert.Equal(t, "*cret", ApplyMask("ecret", "partial"))
	assert.Nil(t, ApplyMask(nil, "partial"))
}

func TestApplyMask_Null(t *testing.T) {
	assert.Nil(t, ApplyMask("secret@email.com", "null"))
	assert.Nil(t, ApplyMask(12345, "null"))
	assert.Nil(t, ApplyMask(nil, "null"))
}

func TestApplyMask_Unknown(t *testing.T) {
	assert.Equal(t, "keep-me", ApplyMask("keep-me", "unknown"))
	assert.Equal(t, "keep-me", ApplyMask("keep-me", ""))
}

func TestMaskRows(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "email": "alice@example.com", "name": "Alice"},
		{"id": 2, "email": "bob@example.com", "name": "Bob"},
	}

	masks := map[string]string{
		"email": "redact",
	}

	MaskRows(rows, masks)

	assert.Equal(t, "***", rows[0]["email"])
	assert.Equal(t, "***", rows[1]["email"])
	assert.Equal(t, "Alice", rows[0]["name"])
	assert.Equal(t, 1, rows[0]["id"])
}

func TestMaskRows_NoMasks(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "email": "alice@example.com"},
	}

	MaskRows(rows, nil)
	assert.Equal(t, "alice@example.com", rows[0]["email"])

	MaskRows(rows, map[string]string{})
	assert.Equal(t, "alice@example.com", rows[0]["email"])
}

func TestMaskRows_MissingColumn(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "name": "Alice"},
	}

	masks := map[string]string{
		"ssn": "redact",
	}

	MaskRows(rows, masks)
	assert.Equal(t, "Alice", rows[0]["name"])
}
