package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileAuditor_CreatesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	fa, err := NewFileAuditor(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, fa.Close()) }()

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestNewFileAuditor_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := NewFileAuditor("/nonexistent/dir/audit.jsonl")
	require.Error(t, err)
}

func TestFileAuditor_Record_WritesNDJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	fa, err := NewFileAuditor(path)
	require.NoError(t, err)

	fa.Record(context.Background(), port.AuditEntry{
		Tool:         "query",
		SQL:          "SELECT 1",
		RowsReturned: 1,
		DurationMS:   42,
	})
	require.NoError(t, fa.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var entry fileEntry
	require.NoError(t, json.Unmarshal(data, &entry))

	assert.Equal(t, "query", entry.Tool)
	assert.Equal(t, "SELECT 1", entry.SQL)
	assert.Equal(t, 1, entry.RowsReturned)
	assert.Equal(t, int64(42), entry.DurationMS)
	assert.Nil(t, entry.Error)
	assert.NotEmpty(t, entry.Timestamp)
}

func TestFileAuditor_Record_WithError(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	fa, err := NewFileAuditor(path)
	require.NoError(t, err)

	fa.Record(context.Background(), port.AuditEntry{
		Tool: "query",
		SQL:  "SELECT bad",
		Err:  fmt.Errorf("syntax error"),
	})
	require.NoError(t, fa.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var entry fileEntry
	require.NoError(t, json.Unmarshal(data, &entry))

	require.NotNil(t, entry.Error)
	assert.Equal(t, "syntax error", *entry.Error)
}

func TestFileAuditor_Record_MultipleEntries(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	fa, err := NewFileAuditor(path)
	require.NoError(t, err)

	for i := range 3 {
		fa.Record(context.Background(), port.AuditEntry{
			Tool:         "query",
			SQL:          fmt.Sprintf("SELECT %d", i),
			RowsReturned: i,
		})
	}
	require.NoError(t, fa.Close())

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	scanner := bufio.NewScanner(f)
	var count int
	for scanner.Scan() {
		var entry fileEntry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		count++
	}
	assert.Equal(t, 3, count)
}

func TestFileAuditor_Record_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	fa, err := NewFileAuditor(path)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			fa.Record(context.Background(), port.AuditEntry{
				Tool: "query",
				SQL:  fmt.Sprintf("SELECT %d", n),
			})
		}(i)
	}
	wg.Wait()
	require.NoError(t, fa.Close())

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	scanner := bufio.NewScanner(f)
	var count int
	for scanner.Scan() {
		var entry fileEntry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry), "line %d: %s", count+1, scanner.Text())
		count++
	}
	assert.Equal(t, 50, count)
}

func TestFileAuditor_Append(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")

	// First auditor writes one entry.
	fa1, err := NewFileAuditor(path)
	require.NoError(t, err)
	fa1.Record(context.Background(), port.AuditEntry{Tool: "query", SQL: "SELECT 1"})
	require.NoError(t, fa1.Close())

	// Second auditor appends another entry.
	fa2, err := NewFileAuditor(path)
	require.NoError(t, err)
	fa2.Record(context.Background(), port.AuditEntry{Tool: "query", SQL: "SELECT 2"})
	require.NoError(t, fa2.Close())

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	scanner := bufio.NewScanner(f)
	var count int
	for scanner.Scan() {
		count++
	}
	assert.Equal(t, 2, count)
}

func TestNoopAuditor(t *testing.T) {
	t.Parallel()
	a := port.NoopAuditor{}
	a.Record(context.Background(), port.AuditEntry{Tool: "query", SQL: "SELECT 1"})
	assert.NoError(t, a.Close())
}
