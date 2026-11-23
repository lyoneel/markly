package markly

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

// docOfSize returns a document with frontmatter and a body padded to
// exactly n bytes.
func docOfSize(t *testing.T, n int) string {
	t.Helper()
	head := "---\ntitle: t\n---\n# H\n"
	if n < len(head)+1 {
		t.Fatalf("size %d too small", n)
	}
	return head + strings.Repeat("x", n-len(head)-1) + "\n"
}

func writeDoc(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultMaxFileSizeExceedsMaxLineLength(t *testing.T) {
	if DefaultMaxFileSize <= MaxLineLength {
		t.Fatalf("DefaultMaxFileSize %d must exceed MaxLineLength %d", DefaultMaxFileSize, MaxLineLength)
	}
}

func TestMaxFileSizeBoundary(t *testing.T) {
	const limit = 200
	dir := t.TempDir()
	at := writeDoc(t, dir, "at.md", docOfSize(t, limit))
	over := writeDoc(t, dir, "over.md", docOfSize(t, limit+1))

	md := NewMDFile(at, WithMaxFileSize(limit))
	if meta, err := md.GetMetadata(); err != nil || meta == nil {
		t.Fatalf("at limit GetMetadata = %v, %v", meta, err)
	}
	if err := md.LoadContent(); err != nil {
		t.Fatalf("at limit LoadContent: %v", err)
	}
	if _, err := NewMDFileWithContent(at, WithMaxFileSize(limit)); err != nil {
		t.Fatalf("at limit NewMDFileWithContent: %v", err)
	}

	md = NewMDFile(over, WithMaxFileSize(limit))
	if _, err := md.GetMetadata(); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("over limit GetMetadata err = %v, want ErrFileTooLarge", err)
	}
	if err := md.LoadContent(); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("over limit LoadContent err = %v, want ErrFileTooLarge", err)
	}
	if _, err := NewMDFileWithContent(over, WithMaxFileSize(limit)); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("over limit NewMDFileWithContent err = %v", err)
	}
	if err := md.LoadContent(); !strings.Contains(err.Error(), over) {
		t.Fatalf("error %q does not name the file", err)
	}
}

// A reader without Stat (or a file that grows after the check) is
// limited while it is read, one byte at a time included.
func TestMaxFileSizeWhileReading(t *testing.T) {
	const limit = 120
	for _, tc := range []struct {
		size    int
		tooLong bool
	}{{limit - 1, false}, {limit, false}, {limit + 1, true}, {limit * 50, true}} {
		doc := docOfSize(t, tc.size)
		md := NewMDFile("stream.md", WithMaxFileSize(limit))
		md.open = func() (io.ReadCloser, error) {
			return io.NopCloser(iotest.OneByteReader(strings.NewReader(doc))), nil
		}
		err := md.LoadContent()
		if got := errors.Is(err, ErrFileTooLarge); got != tc.tooLong {
			t.Fatalf("size %d: LoadContent err = %v, want too large %v", tc.size, err, tc.tooLong)
		}
	}
}

func TestLimitedReaderNeverPassesTheLimit(t *testing.T) {
	const limit = 10
	r := &limitedReadCloser{rc: io.NopCloser(strings.NewReader(strings.Repeat("a", 100))), left: limit, limit: limit, path: "p"}
	got, err := io.ReadAll(r)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if len(got) > limit {
		t.Fatalf("read %d bytes past a limit of %d", len(got), limit)
	}
	if _, err := r.Read(make([]byte, 4)); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("read after the limit err = %v", err)
	}
}

func TestDefaultMaxFileSizeApplies(t *testing.T) {
	p := filepath.Join(t.TempDir(), "huge.md")
	// A sparse file: the size check refuses it without reading it.
	if err := os.WriteFile(p, []byte("---\ntitle: t\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(p, DefaultMaxFileSize+1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMDFile(p).GetMetadata(); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("default limit GetMetadata err = %v", err)
	}
	if err := os.Truncate(p, DefaultMaxFileSize); err != nil {
		t.Fatal(err)
	}
	if meta, err := NewMDFile(p).GetMetadata(); err != nil || meta == nil {
		t.Fatalf("file at the default limit: %v, %v", meta, err)
	}
}

func TestMaxFileSizeZeroOrNegativeDisables(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		md := NewMDFile("x.md", WithMaxFileSize(limit))
		if got := md.sizeLimit(); got != 0 {
			t.Fatalf("WithMaxFileSize(%d): sizeLimit = %d, want 0 (disabled)", limit, got)
		}
		f := io.NopCloser(strings.NewReader("x"))
		if got, err := md.limitFile(f); err != nil || got != f {
			t.Fatalf("WithMaxFileSize(%d) wrapped the reader: %T, %v", limit, got, err)
		}
	}
	if got := NewMDFile("x.md").sizeLimit(); got != DefaultMaxFileSize {
		t.Fatalf("default sizeLimit = %d", got)
	}

	// Disabled, a file over the default limit is read: here it stops at
	// the line-length limit instead of the size limit.
	p := filepath.Join(t.TempDir(), "huge.md")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(p, DefaultMaxFileSize+1); err != nil {
		t.Fatal(err)
	}
	err := NewMDFile(p, WithMaxFileSize(0)).LoadContent()
	if errors.Is(err, ErrFileTooLarge) || !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("disabled limit LoadContent err = %v, want bufio.ErrTooLong only", err)
	}
}

func TestInMemoryDocumentsAreNotLimited(t *testing.T) {
	doc := docOfSize(t, 500)
	md := NewMDFileFromString(doc, WithMaxFileSize(10))
	if meta, err := md.GetMetadata(); err != nil || meta == nil {
		t.Fatalf("GetMetadata = %v, %v", meta, err)
	}
	if _, err := NewMDFileFromBytes([]byte(doc), WithMaxFileSize(10)).GetContent(); err != nil {
		t.Fatal(err)
	}
}

func TestFolderMaxFileSize(t *testing.T) {
	const limit = 200
	dir := t.TempDir()
	writeDoc(t, dir, "small.md", docOfSize(t, limit))
	writeDoc(t, dir, "big.md", docOfSize(t, limit+1))

	f, err := NewMDFolder(dir, WithFolderMaxFileSize(limit))
	if err != nil {
		t.Fatal(err)
	}
	files := f.GetAll()
	if _, ok := files["small.md"]; !ok || len(files) != 1 {
		t.Fatalf("files = %v", files)
	}
	if err := f.Errors()["big.md"]; !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("Errors()[big.md] = %v", err)
	}

	// A file that grows past the limit after discovery fails its lazy load.
	small := files["small.md"]
	fh, err := os.OpenFile(filepath.Join(dir, "small.md"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.Write(bytes.Repeat([]byte("y"), 10)); err != nil {
		t.Fatal(err)
	}
	_ = fh.Close()
	if err := small.LoadContent(); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("grown file LoadContent err = %v", err)
	}

	// The default limit admits both files.
	f, err = NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if files := f.GetAll(); len(files) != 2 || len(f.Errors()) != 0 {
		t.Fatalf("default folder: files %d errors %v", len(files), f.Errors())
	}
	// Zero disables the folder limit as well.
	f, _ = NewMDFolder(dir, WithFolderMaxFileSize(0))
	if files := f.GetAll(); len(files) != 2 {
		t.Fatalf("disabled folder limit: files %d errors %v", len(files), f.Errors())
	}
}
