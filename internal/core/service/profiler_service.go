package service

import (
	"context"

	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// ProfilerService wraps SchemaProfiler for deep table profiling.
type ProfilerService struct {
	profiler port.SchemaProfiler
}

func NewProfilerService(profiler port.SchemaProfiler) *ProfilerService {
	return &ProfilerService{profiler: profiler}
}

func (s *ProfilerService) ProfileTable(ctx context.Context, schema, tableName string) (*port.TableProfile, error) {
	return s.profiler.ProfileTable(ctx, schema, tableName)
}
