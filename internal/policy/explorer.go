package policy

import (
	"context"

	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// PolicyExplorer decorates a SchemaExplorer with policy-based context enrichment.
// It merges business descriptions from the policy YAML into explorer responses.
type PolicyExplorer struct {
	inner  port.SchemaExplorer
	policy *Policy
}

// NewPolicyExplorer wraps an existing SchemaExplorer with context enrichment.
func NewPolicyExplorer(inner port.SchemaExplorer, pol *Policy) *PolicyExplorer {
	return &PolicyExplorer{inner: inner, policy: pol}
}

func (p *PolicyExplorer) ListSchemas(ctx context.Context) ([]port.SchemaInfo, error) {
	return p.inner.ListSchemas(ctx)
}

func (p *PolicyExplorer) ListTables(ctx context.Context) ([]port.TableInfo, error) {
	tables, err := p.inner.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	MergeTableInfoList(tables, p.policy.Context)
	return tables, nil
}

func (p *PolicyExplorer) DescribeTable(ctx context.Context, schema, tableName string) (*port.TableDetail, error) {
	detail, err := p.inner.DescribeTable(ctx, schema, tableName)
	if err != nil {
		return nil, err
	}
	MergeTableDetail(detail, p.policy.Context)
	return detail, nil
}
