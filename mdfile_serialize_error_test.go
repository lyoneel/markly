package markly

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const serializeOrig = "---\ntitle: keep me\ntags: [a, b]\n---\nbody\n"

func assertFileUnchanged(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("file changed to %q, want %q", got, want)
	}
}

func assertOnlyFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Fatalf("directory holds %v, want %v", got, names)
	}
}

func TestSaveFailingSerializerLeavesFileUntouched(t *testing.T) {
	boom := errors.New("boom")
	for _, atomic := range []bool{false, true} {
		dir := t.TempDir()
		path := filepath.Join(dir, "doc.md")
		writeFile(t, path, serializeOrig)

		md := NewMDFile(path)
		meta, err := md.GetMetadata()
		if err != nil {
			t.Fatal(err)
		}
		meta.SetString("title", "changed")
		md.SetSerializer(func(map[string]any) (string, error) { return "", boom })

		if atomic {
			err = md.SaveAtomic()
		} else {
			err = md.Save()
		}
		if !errors.Is(err, ErrMetadataSerialization) || !errors.Is(err, boom) {
			t.Fatalf("atomic=%v: save error = %v; want ErrMetadataSerialization wrapping boom", atomic, err)
		}
		assertFileUnchanged(t, path, serializeOrig)
		assertOnlyFiles(t, dir, "doc.md")
		if !meta.modified {
			t.Fatalf("atomic=%v: failed save cleared the modified flag", atomic)
		}
	}
}

func TestSaveUnencodableYamlValueReturnsError(t *testing.T) {
	for _, atomic := range []bool{false, true} {
		dir := t.TempDir()
		path := filepath.Join(dir, "doc.md")
		writeFile(t, path, serializeOrig)

		md := NewMDFile(path)
		meta, err := md.GetMetadata()
		if err != nil {
			t.Fatal(err)
		}
		meta.Set("bad", func() {})

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("atomic=%v: save panicked: %v", atomic, r)
				}
			}()
			if atomic {
				err = md.SaveAtomic()
			} else {
				err = md.Save()
			}
		}()
		if !errors.Is(err, ErrMetadataSerialization) {
			t.Fatalf("atomic=%v: save error = %v; want ErrMetadataSerialization", atomic, err)
		}
		assertFileUnchanged(t, path, serializeOrig)
		assertOnlyFiles(t, dir, "doc.md")
	}
}

func TestSaveUnencodableTomlValueReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	orig := "+++\ntitle = 'T'\n+++\nbody\n"
	writeFile(t, path, orig)

	md := NewMDFile(path)
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatal(err)
	}
	meta.Set("bad", make(chan int))
	if err := md.Save(); !errors.Is(err, ErrMetadataSerialization) {
		t.Fatalf("Save error = %v; want ErrMetadataSerialization", err)
	}
	assertFileUnchanged(t, path, orig)
}

func TestSaveSerializationErrorDoesNotCreateNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")
	md := NewMDFileFromString("---\ntitle: t\n---\nbody\n")
	meta, _ := md.GetMetadata()
	meta.Set("bad", func() {})
	md.SetPath(path)
	if err := md.Save(); !errors.Is(err, ErrMetadataSerialization) {
		t.Fatalf("Save error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed Save created the file: %v", err)
	}
}

func TestRenderAndSerializeOnSerializationError(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: t\n---\nbody\n")
	meta, _ := md.GetMetadata()

	out, err := md.Render()
	if err != nil || out != "---\ntitle: t\n---\n\nbody\n" {
		t.Fatalf("Render = %q, %v", out, err)
	}

	meta.Set("bad", func() {})
	if _, err := md.Render(); !errors.Is(err, ErrMetadataSerialization) {
		t.Fatalf("Render error = %v; want ErrMetadataSerialization", err)
	}
	if got := md.Serialize(); got != "" {
		t.Fatalf("Serialize = %q; want empty on error", got)
	}
}

func TestCustomSerializerStillUsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	writeFile(t, path, serializeOrig)
	md := NewMDFile(path)
	if _, err := md.GetMetadata(); err != nil {
		t.Fatal(err)
	}
	md.SetSerializer(func(map[string]any) (string, error) { return "title: custom\n", nil })
	if err := md.Save(); err != nil {
		t.Fatal(err)
	}
	assertFileUnchanged(t, path, "---\ntitle: custom\n---\n\nbody\n")
}
