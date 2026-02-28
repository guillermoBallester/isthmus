package policy

import (
	"context"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// PolicyExplorer decorates a SchemaExplorer with policy-based context enrichment.
// It merges business descriptions from the policy YAML into explorer responses
// and applies column masking to sample rows.
type PolicyExplorer struct {
	inner  port.SchemaExplorer
	policy *Policy
	masks  map[string]domain.MaskType
}

// NewPolicyExplorer wraps an existing SchemaExplorer with context enrichment and sample row masking.
func NewPolicyExplorer(inner port.SchemaExplorer, pol *Policy, masks map[string]domain.MaskType) *PolicyExplorer {
	return &PolicyExplorer{inner: inner, policy: pol, masks: masks}
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
	domain.MaskRows(detail.SampleRows, p.masks)
	return detail, nil
}

func (p *PolicyExplorer) Discover(ctx context.Context) (*port.DiscoveryResult, error) {
	result, err := p.inner.Discover(ctx)
	if err != nil {
		return nil, err
	}
	for i := range result.Schemas {
		MergeTableInfoList(result.Schemas[i].Tables, p.policy.Context)
	}
	return result, nil
}
