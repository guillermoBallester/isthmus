package port

import "context"

// AuditEntry represents a single auditable query event.
type AuditEntry struct {
	Tool         string
	SQL          string
	RowsReturned int
	DurationMS   int64
	Err          error
}

// QueryAuditor records query audit events.
type QueryAuditor interface {
	Record(ctx context.Context, entry AuditEntry)
	Close() error
}

// NoopAuditor discards all audit entries.
type NoopAuditor struct{}

func (NoopAuditor) Record(context.Context, AuditEntry) {}
func (NoopAuditor) Close() error                       { return nil }
