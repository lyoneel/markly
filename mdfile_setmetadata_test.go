package markly

import (
	"strings"
	"testing"
)

func TestSetMetadataOnFrontmatterlessDoc(t *testing.T) {
	md := NewMDFileFromString("body only")

	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta != nil {
		t.Fatalf("metadata = %+v, want nil before SetMetadata", meta)
	}

	md.SetMetadata(NewMDMetadataWithFormat(map[string]any{"task_format": "list"}, 0, 0, FMTypeYAML))

	out := md.Serialize()
	if !strings.Contains(out, "---\ntask_format: list\n---") {
		t.Errorf("Serialize output missing frontmatter block:\n%s", out)
	}
	if !strings.Contains(out, "body only") {
		t.Errorf("Serialize output lost body:\n%s", out)
	}
	if md.Type != FMTypeYAML {
		t.Errorf("Type = %q, want yaml", md.Type)
	}
}

func TestSetMetadataKeepsExistingType(t *testing.T) {
	md := NewMDFileFromString("+++\ntitle = \"Toml\"\n+++\n\nbody\n")

	if err := md.SetType(FMTypeTOML); err != nil {
		t.Fatalf("SetType failed: %v", err)
	}
	md.SetMetadata(NewMDMetadataWithFormat(map[string]any{"task_format": "list"}, 0, 0, FMTypeYAML))

	if md.Type != FMTypeTOML {
		t.Errorf("Type = %q, want toml preserved", md.Type)
	}
}

func TestSetMetadataSaveRefreshesLineRange(t *testing.T) {
	md := NewMDFileFromString("body only")
	md.SetMetadata(NewMDMetadataWithFormat(map[string]any{"task_format": "list"}, 0, 0, FMTypeYAML))

	dir := t.TempDir()
	target := dir + "/out.md"
	md.SetPath(target)
	if err := md.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	md2 := NewMDFile(target)
	meta, err := md2.GetMetadata()
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if meta.GetString("task_format") != "list" {
		t.Errorf("task_format = %q, want list", meta.GetString("task_format"))
	}
}
