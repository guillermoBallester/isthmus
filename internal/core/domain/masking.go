package domain

import (
	"crypto/sha256"
	"fmt"
)

// ApplyMask transforms a value according to the mask type.
// Supported mask types: "redact", "hash", "partial", "null".
func ApplyMask(value any, maskType string) any {
	if value == nil {
		return nil
	}

	switch maskType {
	case "redact":
		return "***"
	case "hash":
		s := fmt.Sprintf("%v", value)
		h := sha256.Sum256([]byte(s))
		return fmt.Sprintf("%x", h[:8]) // 16-char hex prefix
	case "partial":
		return maskPartial(value)
	case "null":
		return nil
	default:
		return value
	}
}

// maskPartial reveals only the last 4 characters, replacing the rest with asterisks.
func maskPartial(value any) any {
	s := fmt.Sprintf("%v", value)
	if len(s) <= 4 {
		return "***" + s
	}
	masked := make([]byte, len(s))
	for i := range masked {
		if i < len(s)-4 {
			masked[i] = '*'
		} else {
			masked[i] = s[i]
		}
	}
	return string(masked)
}

// MaskRows applies column masks to query result rows in place.
// The masks map is column-name → mask-type.
func MaskRows(rows []map[string]any, masks map[string]string) {
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
