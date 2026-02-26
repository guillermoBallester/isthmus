package policy

// Policy holds operator-controlled configuration loaded from a YAML file.
// Currently supports data dictionary context; future phases add access rules,
// column masks, row filters, and query templates.
type Policy struct {
	Context ContextConfig `yaml:"context"`
}

// ContextConfig maps fully-qualified table names (schema.table) to
// business descriptions that are merged into MCP tool responses.
type ContextConfig struct {
	Tables map[string]TableContext `yaml:"tables"`
}

// TableContext provides business descriptions for a table and its columns.
type TableContext struct {
	Description string            `yaml:"description"`
	Columns     map[string]string `yaml:"columns"`
}
