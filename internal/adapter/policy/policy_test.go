package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// --- LoadFromFile tests ---

func TestLoadFromFile(t *testing.T) {
	yaml := `
context:
  tables:
    public.users:
      description: "Registered platform users"
      columns:
        mrr: "Monthly Recurring Revenue in cents"
        cac: "Customer Acquisition Cost in USD"
    public.orders:
      description: "Purchase orders"
`
	path := writeTempFile(t, yaml)

	pol, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pol.Context.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(pol.Context.Tables))
	}

	users := pol.Context.Tables["public.users"]
	if users.Description != "Registered platform users" {
		t.Errorf("unexpected description: %q", users.Description)
	}
	if users.Columns["mrr"] != "Monthly Recurring Revenue in cents" {
		t.Errorf("unexpected column desc: %q", users.Columns["mrr"])
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/policy.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	path := writeTempFile(t, "context:\n  tables: [invalid")

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFromFile_EmptyTableKey(t *testing.T) {
	yaml := `
context:
  tables:
    "":
      description: "bad key"
`
	path := writeTempFile(t, yaml)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected validation error for empty table key")
	}
}

func TestLoadFromFile_EmptyColumnKey(t *testing.T) {
	yaml := `
context:
  tables:
    public.users:
      columns:
        "": "bad column key"
`
	path := writeTempFile(t, yaml)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected validation error for empty column key")
	}
}

// --- MergeTableDetail tests ---

func TestMergeTableDetail_MergesWhenEmpty(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {
				Description: "Platform users",
				Columns: map[string]string{
					"email": "User email address",
					"mrr":   "Monthly Recurring Revenue",
				},
			},
		},
	}

	detail := &port.TableDetail{
		Schema:  "public",
		Name:    "users",
		Comment: "", // empty — should be filled
		Columns: []port.ColumnInfo{
			{Name: "id", Comment: ""},
			{Name: "email", Comment: ""}, // should be filled
			{Name: "mrr", Comment: ""},   // should be filled
			{Name: "name", Comment: ""},  // no YAML entry — stays empty
		},
	}

	MergeTableDetail(detail, ctx)

	if detail.Comment != "Platform users" {
		t.Errorf("table comment: got %q, want %q", detail.Comment, "Platform users")
	}
	if detail.Columns[1].Comment != "User email address" {
		t.Errorf("email comment: got %q, want %q", detail.Columns[1].Comment, "User email address")
	}
	if detail.Columns[2].Comment != "Monthly Recurring Revenue" {
		t.Errorf("mrr comment: got %q, want %q", detail.Columns[2].Comment, "Monthly Recurring Revenue")
	}
	if detail.Columns[3].Comment != "" {
		t.Errorf("name comment should be empty, got %q", detail.Columns[3].Comment)
	}
}

func TestMergeTableDetail_DoesNotOverwriteExisting(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {
				Description: "From YAML",
				Columns: map[string]string{
					"email": "From YAML",
				},
			},
		},
	}

	detail := &port.TableDetail{
		Schema:  "public",
		Name:    "users",
		Comment: "From Postgres", // existing — should NOT be overwritten
		Columns: []port.ColumnInfo{
			{Name: "email", Comment: "From Postgres"}, // existing — should NOT be overwritten
		},
	}

	MergeTableDetail(detail, ctx)

	if detail.Comment != "From Postgres" {
		t.Errorf("table comment should not be overwritten: got %q", detail.Comment)
	}
	if detail.Columns[0].Comment != "From Postgres" {
		t.Errorf("column comment should not be overwritten: got %q", detail.Columns[0].Comment)
	}
}

func TestMergeTableDetail_NoMatchingTable(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.orders": {Description: "Orders"},
		},
	}

	detail := &port.TableDetail{
		Schema:  "public",
		Name:    "users",
		Comment: "",
	}

	MergeTableDetail(detail, ctx)

	if detail.Comment != "" {
		t.Errorf("comment should remain empty, got %q", detail.Comment)
	}
}

func TestMergeTableDetail_NilDetail(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {Description: "Users"},
		},
	}
	// Should not panic.
	MergeTableDetail(nil, ctx)
}

// --- MergeTableInfoList tests ---

func TestMergeTableInfoList(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users":  {Description: "Platform users"},
			"public.orders": {Description: "Purchase orders"},
		},
	}

	tables := []port.TableInfo{
		{Schema: "public", Name: "users", Comment: ""},
		{Schema: "public", Name: "orders", Comment: "Existing comment"},
		{Schema: "public", Name: "products", Comment: ""},
	}

	MergeTableInfoList(tables, ctx)

	if tables[0].Comment != "Platform users" {
		t.Errorf("users comment: got %q, want %q", tables[0].Comment, "Platform users")
	}
	if tables[1].Comment != "Existing comment" {
		t.Errorf("orders comment should not be overwritten: got %q", tables[1].Comment)
	}
	if tables[2].Comment != "" {
		t.Errorf("products comment should be empty: got %q", tables[2].Comment)
	}
}

// --- PolicyExplorer tests ---

func TestPolicyExplorer_DescribeTable(t *testing.T) {
	inner := &mockExplorer{
		describeResult: &port.TableDetail{
			Schema: "public",
			Name:   "users",
			Columns: []port.ColumnInfo{
				{Name: "id", Comment: ""},
				{Name: "email", Comment: ""},
			},
		},
	}

	pol := &Policy{
		Context: ContextConfig{
			Tables: map[string]TableContext{
				"public.users": {
					Description: "Registered users",
					Columns: map[string]string{
						"email": "User email",
					},
				},
			},
		},
	}

	pe := NewPolicyExplorer(inner, pol)
	detail, err := pe.DescribeTable(context.Background(), "public", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Comment != "Registered users" {
		t.Errorf("table comment: got %q, want %q", detail.Comment, "Registered users")
	}
	if detail.Columns[1].Comment != "User email" {
		t.Errorf("email comment: got %q, want %q", detail.Columns[1].Comment, "User email")
	}
}

func TestPolicyExplorer_ListTables(t *testing.T) {
	inner := &mockExplorer{
		listTablesResult: []port.TableInfo{
			{Schema: "public", Name: "users", Comment: ""},
		},
	}

	pol := &Policy{
		Context: ContextConfig{
			Tables: map[string]TableContext{
				"public.users": {Description: "Registered users"},
			},
		},
	}

	pe := NewPolicyExplorer(inner, pol)
	tables, err := pe.ListTables(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tables[0].Comment != "Registered users" {
		t.Errorf("comment: got %q, want %q", tables[0].Comment, "Registered users")
	}
}

func TestPolicyExplorer_ListSchemas(t *testing.T) {
	inner := &mockExplorer{
		listSchemasResult: []port.SchemaInfo{{Name: "public"}},
	}

	pol := &Policy{}
	pe := NewPolicyExplorer(inner, pol)

	schemas, err := pe.ListSchemas(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schemas) != 1 || schemas[0].Name != "public" {
		t.Errorf("unexpected schemas: %v", schemas)
	}
}

// --- helpers ---

type mockExplorer struct {
	listSchemasResult []port.SchemaInfo
	listTablesResult  []port.TableInfo
	describeResult    *port.TableDetail
}

func (m *mockExplorer) ListSchemas(_ context.Context) ([]port.SchemaInfo, error) {
	return m.listSchemasResult, nil
}

func (m *mockExplorer) ListTables(_ context.Context) ([]port.TableInfo, error) {
	return m.listTablesResult, nil
}

func (m *mockExplorer) DescribeTable(_ context.Context, _, _ string) (*port.TableDetail, error) {
	return m.describeResult, nil
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
