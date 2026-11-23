//go:build unix

package markly

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// symlinkFixture builds root/vault with note.md, an in-root link
// (alias.md -> note.md), and an escaping link (leak.md -> ../outside/secret.md).
func symlinkFixture(t *testing.T) (vault string) {
	t.Helper()
	root := t.TempDir()
	vault = filepath.Join(root, "vault")
	writeFile(t, filepath.Join(vault, "note.md"), "---\ntitle: note\n---\n# Note\n")
	writeFile(t, filepath.Join(root, "outside", "secret.md"), "---\napi_key: s3cr3t\n---\n# Secret\n")
	if err := os.Symlink("note.md", filepath.Join(vault, "alias.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside", "secret.md"), filepath.Join(vault, "leak.md")); err != nil {
		t.Fatal(err)
	}
	return vault
}

func TestMDFolderDefaultRefusesEscapingSymlink(t *testing.T) {
	folder, err := NewMDFolder(symlinkFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	files := folder.GetAll()
	if _, ok := files["leak.md"]; ok {
		t.Fatal("leak.md was loaded; an escaping symlink must be refused by default")
	}
	if files["note.md"] == nil || files["alias.md"] == nil {
		t.Fatalf("note.md and the in-root alias.md must load, got %v", files)
	}
	if !errors.Is(folder.Errors()["leak.md"], ErrPathEscapesRoot) {
		t.Fatalf("Errors()[leak.md] = %v, want ErrPathEscapesRoot", folder.Errors()["leak.md"])
	}
	meta, _ := files["alias.md"].GetMetadata()
	if meta.GetString("title") != "note" {
		t.Fatalf("alias.md metadata = %v", meta.Data())
	}
}

func TestMDFolderFollowAnyKeepsOldBehavior(t *testing.T) {
	folder, err := NewMDFolder(symlinkFixture(t), WithSymlinkPolicy(FollowAny))
	if err != nil {
		t.Fatal(err)
	}
	files := folder.GetAll()
	leak := files["leak.md"]
	if leak == nil {
		t.Fatalf("FollowAny must read leak.md, errors=%v", folder.Errors())
	}
	meta, _ := leak.GetMetadata()
	if meta.GetString("api_key") != "s3cr3t" {
		t.Fatalf("leak.md metadata = %v", meta.Data())
	}
}

func TestMDFolderNoFollowRefusesAllSymlinks(t *testing.T) {
	folder, err := NewMDFolder(symlinkFixture(t), WithSymlinkPolicy(NoFollow))
	if err != nil {
		t.Fatal(err)
	}
	files := folder.GetAll()
	if len(files) != 1 || files["note.md"] == nil {
		t.Fatalf("NoFollow must load only note.md, got %v", files)
	}
	for _, name := range []string{"alias.md", "leak.md"} {
		if !errors.Is(folder.Errors()[name], ErrSymlinkNotAllowed) {
			t.Fatalf("Errors()[%s] = %v, want ErrSymlinkNotAllowed", name, folder.Errors()[name])
		}
	}
}

func TestMDFolderSymlinkedBaseDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "a.md"), "---\ntitle: a\n---\n")
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
		t.Fatal(err)
	}
	folder, err := NewMDFolder(link)
	if err != nil {
		t.Fatal(err)
	}
	files := folder.GetAll()
	md := files["a.md"]
	if md == nil {
		t.Fatalf("a.md not discovered through a symlinked base, files=%v errors=%v", files, folder.Errors())
	}
	if got, want := md.path, filepath.Join(link, "a.md"); got != want {
		t.Fatalf("path = %q, want the base-path form %q", got, want)
	}
}

func TestMDFolderLazyLoadRechecksPolicy(t *testing.T) {
	vault := symlinkFixture(t)
	folder, err := NewMDFolder(vault)
	if err != nil {
		t.Fatal(err)
	}
	md := folder.GetAll()["note.md"]
	if md == nil {
		t.Fatal("note.md missing")
	}
	// Swap the discovered file for an escaping symlink before the lazy
	// content load; the read must be refused, not follow the new link.
	notePath := filepath.Join(vault, "note.md")
	if err := os.Remove(notePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(filepath.Dir(vault), "outside", "secret.md"), notePath); err != nil {
		t.Fatal(err)
	}
	err = md.LoadContent()
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("LoadContent after swap = %v, want ErrPathEscapesRoot", err)
	}
}
