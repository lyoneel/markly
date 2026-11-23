//go:build unix

package markly

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSaveAtomicPreservesMode(t *testing.T) {
	for _, mode := range []os.FileMode{0o644, 0o640, 0o600, 0o664} {
		dir := t.TempDir()
		path := filepath.Join(dir, "doc.md")
		writeFile(t, path, "---\ntitle: old\n---\nbody\n")
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		if err := editAndSaveAtomic(t, path); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != mode {
			t.Fatalf("mode after SaveAtomic = %v, want %v", fi.Mode().Perm(), mode)
		}
		assertNoTempFiles(t, dir)
	}
}

func TestSaveAtomicNewFileModeMatchesSave(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"atomic.md", "plain.md"} {
		md := NewMDFileFromString("body\n")
		md.SetPath(filepath.Join(dir, name))
		var err error
		if name == "atomic.md" {
			err = md.SaveAtomic()
		} else {
			err = md.Save()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	a, _ := os.Stat(filepath.Join(dir, "atomic.md"))
	p, _ := os.Stat(filepath.Join(dir, "plain.md"))
	if a.Mode().Perm() != p.Mode().Perm() {
		t.Fatalf("new file mode: SaveAtomic %v, Save %v; want equal (DefaultFilePerm minus umask)", a.Mode().Perm(), p.Mode().Perm())
	}
	if a.Mode().Perm()&^DefaultFilePerm != 0 {
		t.Fatalf("new file mode %v exceeds DefaultFilePerm", a.Mode().Perm())
	}
}

func TestSaveAtomicKeepsSymlinkAndUpdatesTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real", "target.md"), "---\ntitle: old\n---\nbody\n")
	if err := os.Chmod(filepath.Join(dir, "real", "target.md"), 0o640); err != nil {
		t.Fatal(err)
	}
	// Two hops: link.md -> mid.md -> real/target.md (relative links).
	if err := os.Symlink("real/target.md", filepath.Join(dir, "mid.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("mid.md", filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := editAndSaveAtomic(t, filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link.md", "mid.md"} {
		fi, err := os.Lstat(filepath.Join(dir, name))
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is no longer a symlink: %v %v", name, fi, err)
		}
	}
	assertFileUnchanged(t, filepath.Join(dir, "real", "target.md"), "---\ntitle: new\n---\n\nbody\n")
	fi, _ := os.Stat(filepath.Join(dir, "real", "target.md"))
	if fi.Mode().Perm() != 0o640 {
		t.Fatalf("target mode = %v, want 0640", fi.Mode().Perm())
	}
	assertNoTempFiles(t, dir)
	assertNoTempFiles(t, filepath.Join(dir, "real"))
}

func TestSaveAtomicDanglingSymlinkCreatesTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("target.md", filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	md := NewMDFileFromString("body\n")
	md.SetPath(filepath.Join(dir, "link.md"))
	if err := md.SaveAtomic(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(filepath.Join(dir, "link.md"))
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link.md was replaced")
	}
	assertFileUnchanged(t, filepath.Join(dir, "target.md"), "body\n")
}

func TestSaveAtomicSymlinkLoopFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("b.md", filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.md", filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}
	md := NewMDFileFromString("body\n")
	md.SetPath(filepath.Join(dir, "a.md"))
	if err := md.SaveAtomic(); err == nil {
		t.Fatal("SaveAtomic through a symlink loop succeeded")
	}
}

func TestSaveAtomicUnwritableDirectoryFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	orig := "---\ntitle: old\n---\nbody\n"
	writeFile(t, path, orig)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := editAndSaveAtomic(t, path); err == nil {
		t.Fatal("SaveAtomic in a read-only directory succeeded")
	}
	assertFileUnchanged(t, path, orig)
	assertNoTempFiles(t, dir)
}

func TestSaveAtomicKeepsOwnership(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	writeFile(t, path, "---\ntitle: old\n---\nbody\n")
	uid, gid := os.Getuid(), os.Getgid()
	if os.Geteuid() == 0 {
		uid, gid = 4242, 4243
		if err := os.Chown(path, uid, gid); err != nil {
			t.Fatal(err)
		}
	}
	if err := editAndSaveAtomic(t, path); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	st := fi.Sys().(*syscall.Stat_t)
	if int(st.Uid) != uid || int(st.Gid) != gid {
		t.Fatalf("owner after SaveAtomic = %d:%d, want %d:%d", st.Uid, st.Gid, uid, gid)
	}
}

func TestMDFolderSaveAtomicGoesThroughFolder(t *testing.T) {
	vault := symlinkFixture(t)
	if err := os.Chmod(filepath.Join(vault, "note.md"), 0o640); err != nil {
		t.Fatal(err)
	}
	folder, err := NewMDFolder(vault)
	if err != nil {
		t.Fatal(err)
	}
	files := folder.GetAll()
	note := files["note.md"]
	meta, _ := note.GetMetadata()
	meta.SetString("title", "edited")
	if err := note.SaveAtomic(); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(filepath.Join(vault, "note.md"))
	if fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", fi.Mode().Perm())
	}
	re, _ := NewMDFile(filepath.Join(vault, "note.md")).GetMetadata()
	if re.GetString("title") != "edited" {
		t.Fatalf("title = %q", re.GetString("title"))
	}
	assertNoTempFiles(t, vault)

	// The in-root alias keeps being a link.
	alias := files["alias.md"]
	am, _ := alias.GetMetadata()
	am.SetString("title", "via alias")
	if err := alias.Save(); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(filepath.Join(vault, "alias.md")); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("alias.md was replaced by a regular file")
	}
}

func TestMDFolderSaveRefusesFileSwappedForEscapingSymlink(t *testing.T) {
	vault := symlinkFixture(t)
	folder, err := NewMDFolder(vault)
	if err != nil {
		t.Fatal(err)
	}
	note := folder.GetAll()["note.md"]
	meta, _ := note.GetMetadata()
	meta.SetString("title", "x")

	secret := filepath.Join(filepath.Dir(vault), "outside", "secret.md")
	before, _ := os.ReadFile(secret)
	if err := os.Remove(filepath.Join(vault, "note.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(vault, "note.md")); err != nil {
		t.Fatal(err)
	}
	for _, save := range []func() error{note.Save, note.SaveAtomic} {
		if err := save(); !errors.Is(err, ErrPathEscapesRoot) {
			t.Fatalf("save through a swapped escaping link = %v; want ErrPathEscapesRoot", err)
		}
	}
	assertFileUnchanged(t, secret, string(before))
}

func TestMDFolderFileSetPathSavesStandalone(t *testing.T) {
	vault := symlinkFixture(t)
	folder, err := NewMDFolder(vault)
	if err != nil {
		t.Fatal(err)
	}
	note := folder.GetAll()["note.md"]
	out := filepath.Join(t.TempDir(), "copy.md")
	note.SetPath(out)
	if err := note.SaveAtomic(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("copy not written: %v", err)
	}
}
