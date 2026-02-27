package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Len(t, pol.Context.Tables, 2)

	users := pol.Context.Tables["public.users"]
	assert.Equal(t, "Registered platform users", users.Description)
	assert.Equal(t, "Monthly Recurring Revenue in cents", users.Columns["mrr"].Description)
	assert.Empty(t, users.Columns["mrr"].Mask)
}

func TestLoadFromFile_WithMasks(t *testing.T) {
	yaml := `
context:
  tables:
    public.customers:
      description: "Customer accounts"
      columns:
        email:
          description: "Customer email"
          mask: "redact"
        ssn:
          mask: "null"
        phone:
          description: "Phone"
          mask: "partial"
        name:
          description: "Full name"
`
	path := writeTempFile(t, yaml)

	pol, err := LoadFromFile(path)
	require.NoError(t, err)

	customers := pol.Context.Tables["public.customers"]
	assert.Equal(t, "redact", customers.Columns["email"].Mask)
	assert.Equal(t, "Customer email", customers.Columns["email"].Description)
	assert.Equal(t, "null", customers.Columns["ssn"].Mask)
	assert.Equal(t, "partial", customers.Columns["phone"].Mask)
	assert.Empty(t, customers.Columns["name"].Mask)
	assert.Equal(t, "Full name", customers.Columns["name"].Description)
}

func TestLoadFromFile_MixedFormats(t *testing.T) {
	yaml := `
context:
  tables:
    public.users:
      columns:
        mrr: "MRR in cents"
        email:
          description: "User email"
          mask: "hash"
`
	path := writeTempFile(t, yaml)

	pol, err := LoadFromFile(path)
	require.NoError(t, err)

	users := pol.Context.Tables["public.users"]
	assert.Equal(t, "MRR in cents", users.Columns["mrr"].Description)
	assert.Empty(t, users.Columns["mrr"].Mask)
	assert.Equal(t, "User email", users.Columns["email"].Description)
	assert.Equal(t, "hash", users.Columns["email"].Mask)
}

func TestLoadFromFile_InvalidMask(t *testing.T) {
	yaml := `
context:
  tables:
    public.users:
      columns:
        email:
          mask: "encrypt"
`
	path := writeTempFile(t, yaml)

	_, err := LoadFromFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
	assert.Contains(t, err.Error(), "encrypt")
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/policy.yaml")
	require.Error(t, err)
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	path := writeTempFile(t, "context:\n  tables: [invalid")

	_, err := LoadFromFile(path)
	require.Error(t, err)
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
	require.Error(t, err)
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
	require.Error(t, err)
}

// --- MergeTableDetail tests ---

func TestMergeTableDetail_MergesWhenEmpty(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {
				Description: "Platform users",
				Columns: map[string]ColumnContext{
					"email": {Description: "User email address"},
					"mrr":   {Description: "Monthly Recurring Revenue"},
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

	assert.Equal(t, "Platform users", detail.Comment)
	assert.Equal(t, "User email address", detail.Columns[1].Comment)
	assert.Equal(t, "Monthly Recurring Revenue", detail.Columns[2].Comment)
	assert.Empty(t, detail.Columns[3].Comment)
}

func TestMergeTableDetail_DoesNotOverwriteExisting(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {
				Description: "From YAML",
				Columns: map[string]ColumnContext{
					"email": {Description: "From YAML"},
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

	assert.Equal(t, "From Postgres", detail.Comment)
	assert.Equal(t, "From Postgres", detail.Columns[0].Comment)
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

	assert.Empty(t, detail.Comment)
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

	assert.Equal(t, "Platform users", tables[0].Comment)
	assert.Equal(t, "Existing comment", tables[1].Comment)
	assert.Empty(t, tables[2].Comment)
}

// --- MaskSpec tests ---

func TestMaskSpec(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {
				Columns: map[string]ColumnContext{
					"email": {Description: "User email", Mask: "redact"},
					"name":  {Description: "Full name"},
				},
			},
			"public.orders": {
				Columns: map[string]ColumnContext{
					"total": {Description: "Order total"},
				},
			},
		},
	}

	spec := MaskSpec(ctx)
	assert.Equal(t, map[string]string{"email": "redact"}, spec)
}

func TestMaskSpec_Empty(t *testing.T) {
	ctx := ContextConfig{
		Tables: map[string]TableContext{
			"public.users": {
				Columns: map[string]ColumnContext{
					"name": {Description: "Full name"},
				},
			},
		},
	}

	spec := MaskSpec(ctx)
	assert.Empty(t, spec)
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
					Columns: map[string]ColumnContext{
						"email": {Description: "User email"},
					},
				},
			},
		},
	}

	pe := NewPolicyExplorer(inner, pol)
	detail, err := pe.DescribeTable(context.Background(), "public", "users")
	require.NoError(t, err)

	assert.Equal(t, "Registered users", detail.Comment)
	assert.Equal(t, "User email", detail.Columns[1].Comment)
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
	require.NoError(t, err)

	assert.Equal(t, "Registered users", tables[0].Comment)
}

func TestPolicyExplorer_ListSchemas(t *testing.T) {
	inner := &mockExplorer{
		listSchemasResult: []port.SchemaInfo{{Name: "public"}},
	}

	pol := &Policy{}
	pe := NewPolicyExplorer(inner, pol)

	schemas, err := pe.ListSchemas(context.Background())
	require.NoError(t, err)
	require.Len(t, schemas, 1)
	assert.Equal(t, "public", schemas[0].Name)
}

// --- MaskingProfiler tests ---

func TestMaskingProfiler(t *testing.T) {
	inner := &mockProfiler{
		result: &port.TableProfile{
			Schema: "public",
			Name:   "users",
			SampleRows: []map[string]any{
				{"id": 1, "email": "alice@example.com", "name": "Alice"},
				{"id": 2, "email": "bob@example.com", "name": "Bob"},
			},
		},
	}

	masks := map[string]string{"email": "redact"}
	mp := NewMaskingProfiler(inner, masks)

	profile, err := mp.ProfileTable(context.Background(), "public", "users")
	require.NoError(t, err)

	assert.Equal(t, "***", profile.SampleRows[0]["email"])
	assert.Equal(t, "***", profile.SampleRows[1]["email"])
	assert.Equal(t, "Alice", profile.SampleRows[0]["name"])
}

func TestMaskingProfiler_NoMasks(t *testing.T) {
	inner := &mockProfiler{
		result: &port.TableProfile{
			SampleRows: []map[string]any{
				{"email": "alice@example.com"},
			},
		},
	}

	mp := NewMaskingProfiler(inner, nil)
	profile, err := mp.ProfileTable(context.Background(), "public", "users")
	require.NoError(t, err)

	assert.Equal(t, "alice@example.com", profile.SampleRows[0]["email"])
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

type mockProfiler struct {
	result *port.TableProfile
}

func (m *mockProfiler) ProfileTable(_ context.Context, _, _ string) (*port.TableProfile, error) {
	return m.result, nil
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
