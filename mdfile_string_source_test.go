package markly

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMDFileFromStringYAML(t *testing.T) {
	content := "---\ntitle: Test\ntags: [a, b]\n---\n\n# Heading\n\nBody text.\n"

	md := NewMDFileFromString(content)

	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta.GetString("title") != "Test" {
		t.Errorf("title = %q, want Test", meta.GetString("title"))
	}
	if tags := meta.GetStringList("tags"); len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags = %v, want [a b]", tags)
	}
	if md.Type != FMTypeYAML {
		t.Errorf("Type = %q, want yaml", md.Type)
	}

	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if len(c.Headings) != 1 || c.Headings[0].Text != "Heading" {
		t.Errorf("headings = %+v, want one Heading", c.Headings)
	}
}

func TestNewMDFileFromStringTOML(t *testing.T) {
	content := "+++\ntitle = \"Test\"\n+++\n\n# Heading\n"

	md := NewMDFileFromString(content)

	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta.GetString("title") != "Test" {
		t.Errorf("title = %q, want Test", meta.GetString("title"))
	}
	if md.Type != FMTypeTOML {
		t.Errorf("Type = %q, want toml", md.Type)
	}
}

func TestNewMDFileFromStringNoFrontmatter(t *testing.T) {
	md := NewMDFileFromString("# Just a heading\n\ntext\n")

	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta != nil {
		t.Errorf("metadata = %+v, want nil", meta)
	}

	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if len(c.Headings) != 1 {
		t.Errorf("headings = %d, want 1", len(c.Headings))
	}
}

func TestNewMDFileFromStringEmpty(t *testing.T) {
	md := NewMDFileFromString("")

	if _, err := md.GetMetadata(); err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if c == nil {
		t.Fatal("content is nil")
	}
	if len(c.Headings) != 0 {
		t.Errorf("headings = %d, want 0", len(c.Headings))
	}
}

func TestStringSourceSetPathAndSave(t *testing.T) {
	content := "---\ntitle: Roundtrip\n---\n\nBody.\n"

	md := NewMDFileFromString(content)
	if err := md.Save(); err == nil {
		t.Fatal("Save without path must fail")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "out.md")
	md.SetPath(target)
	if err := md.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	saved, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}

	// Re-parse the saved file and compare values.
	md2 := NewMDFileFromString(string(saved))
	meta, err := md2.GetMetadata()
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if meta.GetString("title") != "Roundtrip" {
		t.Errorf("saved title = %q, want Roundtrip", meta.GetString("title"))
	}
	c, err := md2.GetContent()
	if err != nil {
		t.Fatalf("re-parse content failed: %v", err)
	}
	if c.RawBody != "\nBody." && c.RawBody != "Body." {
		t.Errorf("saved body = %q", c.RawBody)
	}
}

func TestStringSourceSaveAtomic(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: Atomic\n---\n\nBody.\n")

	dir := t.TempDir()
	target := filepath.Join(dir, "atomic.md")
	md.SetPath(target)
	if err := md.SaveAtomic(); err != nil {
		t.Fatalf("SaveAtomic failed: %v", err)
	}

	saved, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	md2 := NewMDFileFromString(string(saved))
	meta, _ := md2.GetMetadata()
	if meta.GetString("title") != "Atomic" {
		t.Errorf("saved title = %q, want Atomic", meta.GetString("title"))
	}

	// No leftover temp files.
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".md" {
			t.Errorf("leftover file in dir: %s", entry.Name())
		}
	}
}
