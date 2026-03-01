package domain

import (
	"crypto/sha256"
	"fmt"
)

// MaskType represents a column masking strategy.
type MaskType string

const (
	MaskRedact  MaskType = "redact"
	MaskHash    MaskType = "hash"
	MaskPartial MaskType = "partial"
	MaskNull    MaskType = "null"
)

// Valid returns true if the MaskType is a recognised masking strategy
// (including the zero value "", which means "no mask").
func (m MaskType) Valid() bool {
	switch m {
	case MaskRedact, MaskHash, MaskPartial, MaskNull, "":
		return true
	}
	return false
}

// ApplyMask transforms a value according to the mask type.
// Masked values may change type (e.g. int -> string for hash/partial).
// MaskNull returns nil, which is indistinguishable from SQL NULL.
// Column matching is by name only — no table qualification.
func ApplyMask(value any, maskType MaskType) any {
	if value == nil {
		return nil
	}

	switch maskType {
	case MaskRedact:
		return "***"
	case MaskHash:
		s := fmt.Sprintf("%v", value)
		h := sha256.Sum256([]byte(s))
		return fmt.Sprintf("%x", h) // full 256-bit, 64 hex chars
	case MaskPartial:
		return maskPartial(value)
	case MaskNull:
		return nil
	default:
		return value
	}
}

// maskPartial reveals only the last 4 characters, replacing the rest with
// asterisks. Works correctly with multi-byte (unicode) strings.
func maskPartial(value any) string {
	s := fmt.Sprintf("%v", value)
	runes := []rune(s)
	if len(runes) <= 4 {
		return "***" + s
	}
	masked := make([]rune, len(runes))
	for i := range masked {
		if i < len(runes)-4 {
			masked[i] = '*'
		} else {
			masked[i] = runes[i]
		}
	}
	return string(masked)
}

// MaskRows applies column masks to query result rows in place.
// The masks map is column-name -> mask-type.
func MaskRows(rows []map[string]any, masks map[string]MaskType) {
	if len(masks) == 0 {
		return
	}
	for _, row := range rows {
		for col, maskType := range masks {
			if val, exists := row[col]; exists {
				row[col] = ApplyMask(val, maskType)
			}
		}
	}
}

// MaskRowsWithAliases applies column masks to query result rows in place,
// resolving column aliases. For each mask key (original column name), it
// first checks if the row contains that key directly; if not, it checks
// whether the column was aliased and masks the aliased key instead.
func MaskRowsWithAliases(rows []map[string]any, masks map[string]MaskType, aliases map[string]string) {
	if len(masks) == 0 {
		return
	}
	if len(aliases) == 0 {
		MaskRows(rows, masks)
		return
	}
	for _, row := range rows {
		for col, maskType := range masks {
			if val, exists := row[col]; exists {
				row[col] = ApplyMask(val, maskType)
			} else if alias, hasAlias := aliases[col]; hasAlias {
				if val, exists := row[alias]; exists {
					row[alias] = ApplyMask(val, maskType)
				}
			}
		}
	}
}
