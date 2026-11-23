package markly

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReloadDropsDeletedFilesAndFixedErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntitle: a\n---\n")
	writeFile(t, filepath.Join(dir, "bad.md"), "---\ntitle: [\n---\n")
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetAll(); len(got) != 1 || len(f.Errors()) != 1 {
		t.Fatalf("first load: %d files, errors %v", len(got), f.Errors())
	}

	if err := os.Remove(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "bad.md"), "---\ntitle: fixed\n---\n")

	got := f.GetAll()
	if len(got) != 1 || got["bad.md"] == nil {
		t.Fatalf("second load = %v", got)
	}
	if len(f.Errors()) != 0 {
		t.Fatalf("Errors() after the fix = %v; want none", f.Errors())
	}
	n := 0
	f.Iterate(func(md *MDFile) {
		n++
		if filepath.Base(md.path) != "bad.md" {
			t.Fatalf("Iterate visited %s", md.path)
		}
	})
	if n != 1 {
		t.Fatalf("Iterate visited %d files, want 1", n)
	}
	if res := f.GetByMetadata(map[string]any{"title": "a"}); len(res) != 0 {
		t.Fatalf("GetByMetadata still finds the deleted file: %v", res)
	}
	order, err := f.GetLoadOrder()
	if err != nil || len(order) != 1 {
		t.Fatalf("GetLoadOrder = %v, %v", order, err)
	}
}

func TestReloadDropsStaleDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ndepends: [b.md]\n---\n")
	writeFile(t, filepath.Join(dir, "b.md"), "---\ndepends: [a.md]\n---\n")
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.GetAll()
	if _, err := f.GetLoadOrder(); err == nil {
		t.Fatal("expected a cycle on the first load")
	}
	writeFile(t, filepath.Join(dir, "b.md"), "---\ntitle: b\n---\n")
	f.GetAll()
	if _, err := f.GetLoadOrder(); err != nil {
		t.Fatalf("cycle still reported after reload: %v", err)
	}
}

func TestReloadNewErrorReplacesOld(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntitle: [\n---\n")
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.GetAll()
	if err := os.Rename(filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}
	f.GetAll()
	if _, ok := f.Errors()["a.md"]; ok || f.Errors()["b.md"] == nil {
		t.Fatalf("Errors() = %v; want only b.md", f.Errors())
	}
}
