package markly

import (
	"os"
	"path/filepath"
	"testing"
)

func setupFolderFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"visible.md":        "---\ntitle: Visible\n---\n\nbody\n",
		"nested/inner.md":   "---\ntitle: Inner\n---\n\nbody\n",
		".hidden/secret.md": "---\ntitle: Secret\n---\n\nbody\n",
		"broken/bad.md":     "---\ntitle: [unclosed\n---\n\nbody\n",
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func TestMDFolderDefaultIncludesDotDirs(t *testing.T) {
	dir := setupFolderFixture(t)

	folder, err := NewMDFolder(dir)
	if err != nil {
		t.Fatalf("NewMDFolder: %v", err)
	}
	all := folder.GetAll()

	if _, ok := all[filepath.FromSlash(".hidden/secret.md")]; !ok {
		t.Error("default mode must include dot-directory files")
	}
	if _, ok := all["visible.md"]; !ok {
		t.Error("visible.md missing")
	}
}

func TestMDFolderSkipDotDirs(t *testing.T) {
	dir := setupFolderFixture(t)

	folder, err := NewMDFolder(dir, WithSkipDotDirs(true))
	if err != nil {
		t.Fatalf("NewMDFolder: %v", err)
	}
	all := folder.GetAll()

	for path := range all {
		if pathHasDotDir(path) {
			t.Errorf("skip mode returned dot-dir file: %s", path)
		}
	}
	if _, ok := all["visible.md"]; !ok {
		t.Error("visible.md missing")
	}
	if _, ok := all[filepath.FromSlash("nested/inner.md")]; !ok {
		t.Error("nested/inner.md missing")
	}
}

func TestMDFolderErrorsCollected(t *testing.T) {
	dir := setupFolderFixture(t)

	folder, err := NewMDFolder(dir)
	if err != nil {
		t.Fatalf("NewMDFolder: %v", err)
	}
	all := folder.GetAll()

	if _, ok := all[filepath.FromSlash("broken/bad.md")]; ok {
		t.Error("broken file must be skipped from results")
	}
	errs := folder.Errors()
	if len(errs) == 0 {
		t.Fatal("expected collected metadata errors, got none")
	}
	found := false
	for path := range errs {
		if path == filepath.FromSlash("broken/bad.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("error for broken/bad.md not collected; errors: %v", errs)
	}
}
