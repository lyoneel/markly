package validate

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/lyoneel/markly"
)

func sparseFile(t *testing.T, name, head string, size int64) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(head), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(p, size); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOversizedDocumentIsRefused(t *testing.T) {
	p := sparseFile(t, "doc.md", "---\ntitle: t\n---\n", markly.DefaultMaxFileSize+1)
	if _, err := documentFrontmatter(p); !errors.Is(err, markly.ErrFileTooLarge) {
		t.Fatalf("documentFrontmatter err = %v", err)
	}
	if _, err := LoadSchema(p); !errors.Is(err, markly.ErrFileTooLarge) || !errors.Is(err, ErrSchema) {
		t.Fatalf("LoadSchema err = %v", err)
	}
}

func TestReadLimitedStopsAGrowingStream(t *testing.T) {
	dirInfo, err := os.Stat(t.TempDir()) // not a regular file: no size check
	if err != nil {
		t.Fatal(err)
	}
	limit := markly.DefaultMaxFileSize
	if _, err := readLimited(io.LimitReader(zeros{}, limit+1), dirInfo, "s"); !errors.Is(err, markly.ErrFileTooLarge) {
		t.Fatalf("over limit err = %v", err)
	}
	raw, err := readLimited(strings.NewReader("abc"), dirInfo, "s")
	if err != nil || string(raw) != "abc" {
		t.Fatalf("small read = %q, %v", raw, err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
