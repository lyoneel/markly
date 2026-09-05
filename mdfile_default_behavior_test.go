package markly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultBehaviorYAMLRoundTrip asserts that default mode keeps the
// pre-vault-manipulation behavior: lazy content loading, yaml.Marshal
// serialization, headings counted inside fences, and typed timestamp
// values.
func TestDefaultBehaviorYAMLRoundTrip(t *testing.T) {
	content := "---\ntitle: Regression\ncreated: 2026-01-02\ntags:\n  - a\n  - b\n---\n\n# Heading\n\n```\n# fenced\n```\n\nbody text\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "regress.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	md := NewMDFile(path)

	// Lazy loading: content unavailable until requested.
	if _, err := md.GetContent(); err == nil {
		t.Error("lazy mode must not auto-load content")
	}

	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta.GetString("title") != "Regression" {
		t.Errorf("title = %q", meta.GetString("title"))
	}
	if meta.FromLine != 2 {
		t.Errorf("FromLine = %d, want 2", meta.FromLine)
	}

	// Default mode resolves dates to time.Time (yaml.v3 typing), not
	// raw strings.
	if _, isString := meta.Get("created").(string); isString {
		t.Error("default mode must type date scalars, not keep raw strings")
	}

	if err := md.LoadContent(); err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}
	c, _ := md.GetContent()
	// Default mode counts fenced headings: Heading, fenced.
	if len(c.Headings) != 2 {
		t.Errorf("default headings = %d, want 2 (fenced included)", len(c.Headings))
	}
}

// TestDefaultBehaviorSaveUnmodified verifies that saving an unmodified
// file keeps frontmatter keys and body content intact.
func TestDefaultBehaviorSaveUnmodified(t *testing.T) {
	content := "---\ntitle: Stable\n---\n\n# Heading\n\nbody\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "stable.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	md, err := NewMDFileWithContent(path)
	if err != nil {
		t.Fatalf("NewMDFileWithContent: %v", err)
	}
	if err := md.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(saved)

	if !strings.Contains(out, "title: Stable") {
		t.Errorf("frontmatter lost after save:\n%s", out)
	}
	if !strings.Contains(out, "# Heading") {
		t.Errorf("heading lost after save:\n%s", out)
	}
	if !strings.Contains(out, "body") {
		t.Errorf("body lost after save:\n%s", out)
	}

	// No serializer set: output uses yaml.Marshal formatting.
	if !strings.HasPrefix(out, "---\n") {
		t.Errorf("output must start with frontmatter delimiter:\n%s", out)
	}
}

// TestDefaultBehaviorTOML verifies TOML round-trip stays unchanged.
func TestDefaultBehaviorTOML(t *testing.T) {
	content := "+++\ntitle = \"TOML Doc\"\n+++\n\n# H\n\nbody\n"

	md := NewMDFileFromString(content)
	if md.Type != FMTypeTOML {
		t.Fatalf("Type = %q, want toml", md.Type)
	}
	meta, _ := md.GetMetadata()
	if meta.GetString("title") != "TOML Doc" {
		t.Errorf("title = %q", meta.GetString("title"))
	}
	// Custom serializer does not apply to TOML.
	md.SetSerializer(func(map[string]any) (string, error) {
		return "injected: true", nil
	})
	out := md.Serialize()
	if strings.Contains(out, "injected") {
		t.Error("custom serializer must not apply to TOML documents")
	}
	if !strings.Contains(out, "title") {
		t.Errorf("TOML output missing title:\n%s", out)
	}
}
