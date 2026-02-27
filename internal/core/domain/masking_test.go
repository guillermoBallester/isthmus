package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskType_Valid(t *testing.T) {
	t.Parallel()
	valid := []MaskType{"", MaskRedact, MaskHash, MaskPartial, MaskNull}
	for _, mt := range valid {
		assert.True(t, mt.Valid(), "expected %q to be valid", mt)
	}

	invalid := []MaskType{"encrypt", "REDACT", "mask", "sha256"}
	for _, mt := range invalid {
		assert.False(t, mt.Valid(), "expected %q to be invalid", mt)
	}
}

func TestApplyMask_Redact(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "***", ApplyMask("secret@email.com", MaskRedact))
	assert.Equal(t, "***", ApplyMask(12345, MaskRedact))
	assert.Equal(t, "***", ApplyMask(3.14, MaskRedact))
	assert.Equal(t, "***", ApplyMask("", MaskRedact))
	assert.Nil(t, ApplyMask(nil, MaskRedact))
}

func TestApplyMask_Hash(t *testing.T) {
	t.Parallel()
	result := ApplyMask("secret@email.com", MaskHash)
	s, ok := result.(string)
	assert.True(t, ok)
	assert.Len(t, s, 64, "hash should be 64 hex chars (full SHA256)")

	// Deterministic: same input -> same hash.
	assert.Equal(t, result, ApplyMask("secret@email.com", MaskHash))

	// Different input -> different hash.
	assert.NotEqual(t, result, ApplyMask("other@email.com", MaskHash))

	assert.Nil(t, ApplyMask(nil, MaskHash))
}

func TestApplyMask_Hash_EmptyString(t *testing.T) {
	t.Parallel()
	result := ApplyMask("", MaskHash)
	s, ok := result.(string)
	assert.True(t, ok)
	assert.Len(t, s, 64)
}

func TestApplyMask_Hash_DeterminismAcrossTypes(t *testing.T) {
	t.Parallel()
	// int and string of same value produce the same hash because
	// both are formatted via fmt.Sprintf("%v", ...).
	intHash := ApplyMask(12345, MaskHash)
	strHash := ApplyMask("12345", MaskHash)
	assert.Equal(t, intHash, strHash, "int and string with same repr should hash identically")
}

func TestApplyMask_Hash_NumericTypes(t *testing.T) {
	t.Parallel()
	result := ApplyMask(int64(99), MaskHash)
	s, ok := result.(string)
	assert.True(t, ok)
	assert.Len(t, s, 64)

	result2 := ApplyMask(float64(3.14), MaskHash)
	s2, ok := result2.(string)
	assert.True(t, ok)
	assert.Len(t, s2, 64)
}

func TestApplyMask_Partial(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "******7890", ApplyMask("1234567890", MaskPartial))
	assert.Equal(t, "***ab", ApplyMask("ab", MaskPartial))
	assert.Equal(t, "***abcd", ApplyMask("abcd", MaskPartial))
	assert.Equal(t, "*cret", ApplyMask("ecret", MaskPartial))
	assert.Nil(t, ApplyMask(nil, MaskPartial))
}

func TestApplyMask_Partial_EmptyString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "***", ApplyMask("", MaskPartial))
}

func TestApplyMask_Partial_Unicode(t *testing.T) {
	t.Parallel()
	// "café résumé" is 11 runes; last 4 = "sumé"
	result := ApplyMask("café résumé", MaskPartial)
	s, ok := result.(string)
	assert.True(t, ok)
	assert.True(t, strings.HasSuffix(s, "sumé"), "should end with last 4 runes")
	// First 7 runes should be asterisks.
	runes := []rune(s)
	assert.Len(t, runes, 11, "rune count should match original")
	for i := 0; i < 7; i++ {
		assert.Equal(t, '*', runes[i])
	}
}

func TestApplyMask_Partial_VeryLongString(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 10_000)
	result := ApplyMask(long, MaskPartial)
	s, ok := result.(string)
	assert.True(t, ok)
	assert.Len(t, s, 10_000)
	assert.True(t, strings.HasSuffix(s, "aaaa"))
	assert.True(t, strings.HasPrefix(s, "****"))
}

func TestApplyMask_Partial_Numerics(t *testing.T) {
	t.Parallel()
	// int -> "12345" -> "*2345" (wait, len=5, last 4 = "2345")
	result := ApplyMask(12345, MaskPartial)
	assert.Equal(t, "*2345", result)

	result2 := ApplyMask(int64(9876543210), MaskPartial)
	s, ok := result2.(string)
	assert.True(t, ok)
	assert.True(t, strings.HasSuffix(s, "3210"))
}

func TestApplyMask_Null(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ApplyMask("secret@email.com", MaskNull))
	assert.Nil(t, ApplyMask(12345, MaskNull))
	assert.Nil(t, ApplyMask(3.14, MaskNull))
	assert.Nil(t, ApplyMask("", MaskNull))
	assert.Nil(t, ApplyMask(nil, MaskNull))
}

func TestApplyMask_Unknown(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "keep-me", ApplyMask("keep-me", "unknown"))
	assert.Equal(t, "keep-me", ApplyMask("keep-me", ""))
}

func TestMaskRows(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{"id": 1, "email": "alice@example.com", "name": "Alice"},
		{"id": 2, "email": "bob@example.com", "name": "Bob"},
	}

	masks := map[string]MaskType{
		"email": MaskRedact,
	}

	MaskRows(rows, masks)

	assert.Equal(t, "***", rows[0]["email"])
	assert.Equal(t, "***", rows[1]["email"])
	assert.Equal(t, "Alice", rows[0]["name"])
	assert.Equal(t, 1, rows[0]["id"])
}

func TestMaskRows_NoMasks(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{"id": 1, "email": "alice@example.com"},
	}

	MaskRows(rows, nil)
	assert.Equal(t, "alice@example.com", rows[0]["email"])

	MaskRows(rows, map[string]MaskType{})
	assert.Equal(t, "alice@example.com", rows[0]["email"])
}

func TestMaskRows_MissingColumn(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{"id": 1, "name": "Alice"},
	}

	masks := map[string]MaskType{
		"ssn": MaskRedact,
	}

	MaskRows(rows, masks)
	assert.Equal(t, "Alice", rows[0]["name"])
}
