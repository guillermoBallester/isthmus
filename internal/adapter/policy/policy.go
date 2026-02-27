package policy

import (
	"fmt"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"gopkg.in/yaml.v3"
)

// Policy holds operator-controlled configuration loaded from a YAML file.
// Supports data dictionary context and column-level PII masking.
type Policy struct {
	Context ContextConfig `yaml:"context"`
}

// ContextConfig maps fully-qualified table names (schema.table) to
// business descriptions that are merged into MCP tool responses.
type ContextConfig struct {
	Tables map[string]TableContext `yaml:"tables"`
}

// TableContext provides business descriptions and masking rules for a table and its columns.
type TableContext struct {
	Description string                   `yaml:"description"`
	Columns     map[string]ColumnContext `yaml:"columns"`
}

// ColumnContext holds a column's business description and optional mask directive.
type ColumnContext struct {
	Description string          `yaml:"description"`
	Mask        domain.MaskType `yaml:"mask,omitempty"`
}

// UnmarshalYAML supports both the new struct format and the legacy plain-string format.
//
//	columns:
//	  email: "User email"           # legacy: plain string → ColumnContext{Description: "User email"}
//	  ssn:                          # new: struct with optional mask
//	    description: "SSN"
//	    mask: "redact"
func (cc *ColumnContext) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		cc.Description = value.Value
		return nil
	}
	// Decode as struct (avoid infinite recursion by using an alias type).
	type alias ColumnContext
	var a alias
	if err := value.Decode(&a); err != nil {
		return fmt.Errorf("decoding column context: %w", err)
	}
	*cc = ColumnContext(a)
	return nil
}
