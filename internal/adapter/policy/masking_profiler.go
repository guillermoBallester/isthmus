package policy

import (
	"context"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// MaskingProfiler decorates a SchemaProfiler to mask SampleRows in profile results.
type MaskingProfiler struct {
	inner port.SchemaProfiler
	masks map[string]domain.MaskType
}

// NewMaskingProfiler wraps an existing SchemaProfiler with column masking.
func NewMaskingProfiler(inner port.SchemaProfiler, masks map[string]domain.MaskType) *MaskingProfiler {
	return &MaskingProfiler{inner: inner, masks: masks}
}

func (p *MaskingProfiler) ProfileTable(ctx context.Context, schema, tableName string) (*port.TableProfile, error) {
	profile, err := p.inner.ProfileTable(ctx, schema, tableName)
	if err != nil {
		return nil, err
	}

	domain.MaskRows(profile.SampleRows, p.masks)
	return profile, nil
}
