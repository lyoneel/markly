package markly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// editAndSaveAtomic loads path, changes the title, and saves atomically.
func editAndSaveAtomic(t *testing.T, path string) error {
	t.Helper()
	md := NewMDFile(path)
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatal(err)
	}
	meta.SetString("title", "new")
	return md.SaveAtomic()
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file %s", e.Name())
		}
	}
}

func TestSaveAtomicWritesContentAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	writeFile(t, path, "---\ntitle: old\n---\nbody\n")
	if err := editAndSaveAtomic(t, path); err != nil {
		t.Fatal(err)
	}
	assertFileUnchanged(t, path, "---\ntitle: new\n---\n\nbody\n")
	assertNoTempFiles(t, dir)
}

func TestSaveAtomicNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")
	md := NewMDFileFromString("---\ntitle: t\n---\nbody\n")
	md.SetPath(path)
	if err := md.SaveAtomic(); err != nil {
		t.Fatal(err)
	}
	assertFileUnchanged(t, path, "---\ntitle: t\n---\n\nbody\n")
	assertNoTempFiles(t, dir)
}

func TestSaveAtomicMissingDirectoryFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "doc.md")
	md := NewMDFileFromString("body\n")
	md.SetPath(path)
	if err := md.SaveAtomic(); err == nil {
		t.Fatal("SaveAtomic into a missing directory succeeded")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Fatalf("SaveAtomic created the directory: %v", err)
	}
	assertOnlyFiles(t, dir)
}

func TestSaveAtomicRefusesDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "doc.md")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	md := NewMDFileFromString("body\n")
	md.SetPath(target)
	if err := md.SaveAtomic(); err == nil {
		t.Fatal("SaveAtomic over a directory succeeded")
	}
	assertNoTempFiles(t, dir)
}
