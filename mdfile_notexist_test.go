package markly

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertNotExist(t *testing.T, what string, err error) {
	t.Helper()
	if !os.IsNotExist(err) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s = %v; want os.IsNotExist and errors.Is(os.ErrNotExist)", what, err)
	}
	if !strings.HasPrefix(err.Error(), "metadata not found") {
		t.Fatalf("%s message = %q", what, err)
	}
}

func TestMissingFileErrorsMatchIsNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")

	_, err := NewMDFile(path).GetMetadata()
	assertNotExist(t, "GetMetadata", err)

	assertNotExist(t, "LoadContent", NewMDFile(path).LoadContent())

	_, err = NewMDFileWithContent(path)
	assertNotExist(t, "NewMDFileWithContent", err)

	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Path != path {
		t.Fatalf("error %v does not name the path %s", err, path)
	}
}

func TestPermissionErrorKeepsCause(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission bits")
	}
	path := filepath.Join(t.TempDir(), "locked.md")
	writeFile(t, path, "---\ntitle: t\n---\n")
	if err := os.Chmod(path, 0); err != nil {
		t.Skip(err)
	}
	_, err := NewMDFile(path).GetMetadata()
	if !os.IsPermission(err) || !errors.Is(err, os.ErrPermission) || !strings.HasPrefix(err.Error(), "open failed") {
		t.Fatalf("GetMetadata = %v; want a permission error", err)
	}
}

func TestNewMDFolderMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nope")
	f, err := NewMDFolder(dir)
	if f != nil || !os.IsNotExist(err) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewMDFolder(missing) = %v, %v; want nil and a not-exist error", f, err)
	}
}

func TestNewMDFolderOnFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.md")
	writeFile(t, file, "x\n")
	f, err := NewMDFolder(file)
	if f != nil || !errors.Is(err, ErrNotADirectory) {
		t.Fatalf("NewMDFolder(file) = %v, %v; want ErrNotADirectory", f, err)
	}
}

func TestMDFolderReportsDiscoveryFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntitle: a\n---\n")
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The folder disappears after NewMDFolder, so discovery fails.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	files := f.GetAll()
	if files == nil || len(files) != 0 {
		t.Fatalf("GetAllByGlob = %v; want an empty, non-nil map", files)
	}
	if err := f.Errors()[""]; !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Errors() = %v; want the discovery error under \"\"", f.Errors())
	}
}

func TestMDFolderSkipsFileRemovedAfterDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntitle: a\n---\n")
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	md := f.GetAll()["a.md"]
	if err := os.Remove(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	err = md.LoadContent()
	if !os.IsNotExist(err) {
		t.Fatalf("LoadContent after removal = %v; want not-exist", err)
	}
}
