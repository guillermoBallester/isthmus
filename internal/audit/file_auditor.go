package audit

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/port"
)

// fileEntry is the NDJSON-serializable form of an audit record.
type fileEntry struct {
	Timestamp    string  `json:"ts"`
	Tool         string  `json:"tool"`
	SQL          string  `json:"sql"`
	RowsReturned int     `json:"rows_returned"`
	DurationMS   int64   `json:"duration_ms"`
	Error        *string `json:"error"`
}

// FileAuditor writes audit entries as NDJSON (one JSON object per line) to a file.
type FileAuditor struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// NewFileAuditor opens (or creates) the file at path for append-only writing.
func NewFileAuditor(path string) (*FileAuditor, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &FileAuditor{
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

func (a *FileAuditor) Record(_ context.Context, entry port.AuditEntry) {
	fe := fileEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Tool:         entry.Tool,
		SQL:          entry.SQL,
		RowsReturned: entry.RowsReturned,
		DurationMS:   entry.DurationMS,
	}
	if entry.Err != nil {
		s := entry.Err.Error()
		fe.Error = &s
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.enc.Encode(fe) // best-effort; don't fail the request for audit I/O
}

func (a *FileAuditor) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}

// NoopAuditor discards all audit entries.
type NoopAuditor struct{}

func (NoopAuditor) Record(context.Context, port.AuditEntry) {}
func (NoopAuditor) Close() error                            { return nil }
