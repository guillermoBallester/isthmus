package policy

import (
	"fmt"
	"os"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"gopkg.in/yaml.v3"
)

// LoadFromFile reads a YAML policy file and returns a validated Policy.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy file: %w", err)
	}

	var pol Policy
	if err := yaml.Unmarshal(data, &pol); err != nil {
		return nil, fmt.Errorf("parsing policy YAML: %w", err)
	}

	if err := validate(&pol); err != nil {
		return nil, fmt.Errorf("validating policy: %w", err)
	}

	return &pol, nil
}

func validate(pol *Policy) error {
	type maskOrigin struct {
		mask  domain.MaskType
		table string
	}
	seen := make(map[string]maskOrigin)

	for key, tc := range pol.Context.Tables {
		if key == "" {
			return fmt.Errorf("context.tables contains an empty key")
		}
		for col, cc := range tc.Columns {
			if col == "" {
				return fmt.Errorf("context.tables[%q].columns contains an empty key", key)
			}
			if !cc.Mask.Valid() {
				return fmt.Errorf("context.tables[%q].columns[%q].mask: invalid value %q (allowed: redact, hash, partial, null)", key, col, cc.Mask)
			}
			if cc.Mask == "" {
				continue
			}
			if prev, exists := seen[col]; exists && prev.mask != cc.Mask {
				return fmt.Errorf(
					"column %q has conflicting masks: %q in %s vs %q in %s",
					col, prev.mask, prev.table, cc.Mask, key,
				)
			}
			seen[col] = maskOrigin{cc.Mask, key}
		}
	}
	return nil
}
