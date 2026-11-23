package markly

import (
	"bufio"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestLongLinesLoad(t *testing.T) {
	long := strings.Repeat("A", 200*1024)
	cases := map[string]string{
		"body":        "---\ntitle: T\n---\n# Pic\n\n![x](data:image/png;base64," + long + ")\n",
		"frontmatter": "---\ntitle: T\nthumb: " + long + "\n---\nbody\n",
		"no fm":       "# Pic\n\n" + long + "\n",
	}
	for name, doc := range cases {
		path := filepath.Join(t.TempDir(), "img.md")
		writeFile(t, path, doc)
		md := NewMDFile(path)
		if _, err := md.GetMetadata(); err != nil {
			t.Fatalf("%s: GetMetadata: %v", name, err)
		}
		if err := md.LoadContent(); err != nil {
			t.Fatalf("%s: LoadContent: %v", name, err)
		}
		c, _ := md.GetContent()
		if !strings.Contains(c.RawBody, "body") && !strings.Contains(c.RawBody, long) {
			t.Fatalf("%s: body lost", name)
		}
	}
	meta, _ := NewMDFileFromString(cases["frontmatter"]).GetMetadata()
	if meta == nil || len(meta.GetString("thumb")) != len(long) {
		t.Fatal("long frontmatter value lost")
	}
}

func TestMDFolderKeepsLongLineFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "img.md"), "# Pic\n\n![x](data:image/png;base64,"+strings.Repeat("A", 70000)+")\n")
	writeFile(t, filepath.Join(dir, "plain.md"), "---\ntitle: p\n---\n")
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if files := f.GetAll(); len(files) != 2 || len(f.Errors()) != 0 {
		t.Fatalf("files = %d, errors = %v; want 2 files and no errors", len(files), f.Errors())
	}
}

func TestLineOverMaxLengthFails(t *testing.T) {
	doc := "---\ntitle: T\n---\n" + strings.Repeat("A", MaxLineLength+10) + "\n"
	md := NewMDFileFromString(doc)
	err := md.LoadContent()
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("LoadContent = %v; want bufio.ErrTooLong", err)
	}
	_, err = NewMDFileFromString("---\nk: " + strings.Repeat("A", MaxLineLength+10) + "\n---\n").GetMetadata()
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("GetMetadata = %v; want bufio.ErrTooLong", err)
	}
}
