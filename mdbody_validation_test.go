package markly

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const editDoc = "---\ntitle: T\n---\n\n# Title\n\n## Notes\n\n- status: open\n\n- [ ] task one\n"

func TestSingleLineEditsRejectLineBreaks(t *testing.T) {
	bad := []string{"a\nb", "a\rb", "---\nrole: admin\n---", "x\r\n"}
	calls := map[string]func(md *MDFile, s string) error{
		"AppendLine":                    func(md *MDFile, s string) error { return md.AppendLine(s) },
		"InsertLine":                    func(md *MDFile, s string) error { return md.InsertLine(5, s) },
		"SetSectionHeading":             func(md *MDFile, s string) error { return md.SetSectionHeading("notes", s) },
		"SetSectionBullet key":          func(md *MDFile, s string) error { return md.SetSectionBullet("notes", s, "v") },
		"SetSectionBullet value":        func(md *MDFile, s string) error { return md.SetSectionBullet("notes", "k", s) },
		"InsertLineUnderHeading line":   func(md *MDFile, s string) error { return md.InsertLineUnderHeading("Notes", s, true) },
		"InsertLineUnderHeading header": func(md *MDFile, s string) error { return md.InsertLineUnderHeading(s, "x", true) },
		"SetFirstHeading":               func(md *MDFile, s string) error { return md.SetFirstHeading(s) },
		"RewriteCheckboxTitle":          func(md *MDFile, s string) error { return md.RewriteCheckboxTitle(15, s) },
	}
	for name, call := range calls {
		for _, s := range bad {
			md := NewMDFileFromString(editDoc)
			before := md.Serialize()
			if err := call(md, s); !errors.Is(err, ErrMultilineText) {
				t.Fatalf("%s(%q) = %v; want ErrMultilineText", name, s, err)
			}
			if after := md.Serialize(); after != before {
				t.Fatalf("%s(%q) changed the document:\n%q", name, s, after)
			}
		}
	}
}

func TestSingleLineEditsStillWork(t *testing.T) {
	md := NewMDFileFromString(editDoc)
	if err := md.SetFirstHeading("New title"); err != nil {
		t.Fatal(err)
	}
	if err := md.SetSectionBullet("notes", "status", "done"); err != nil {
		t.Fatal(err)
	}
	if err := md.AppendLine("tail"); err != nil {
		t.Fatal(err)
	}
	want := "---\ntitle: T\n---\n\n# New title\n\n## Notes\n\n- status: done\n\n- [ ] task one\ntail\n"
	if got := md.Serialize(); got != want {
		t.Fatalf("Serialize = %q, want %q", got, want)
	}
}

func TestInsertLineCannotInjectFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	writeFile(t, path, "hello\n")
	md := NewMDFile(path)
	if err := md.InsertLine(1, "---\nrole: admin\n---"); !errors.Is(err, ErrMultilineText) {
		t.Fatalf("InsertLine = %v; want ErrMultilineText", err)
	}
	if err := md.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello\n" {
		t.Fatalf("file = %q", got)
	}
	meta, err := NewMDFile(path).GetMetadata()
	if err != nil || meta != nil {
		t.Fatalf("reloaded metadata = %v, %v; want none", meta, err)
	}
}

func TestSetCheckboxMarkerValidation(t *testing.T) {
	for _, marker := range []byte{'\n', '\r', ']', '[', 'a', 0, '-'} {
		md := NewMDFileFromString("- [ ] task\n")
		if err := md.SetCheckboxMarker(1, marker); !errors.Is(err, ErrInvalidCheckboxMarker) {
			t.Fatalf("SetCheckboxMarker(%q) = %v; want ErrInvalidCheckboxMarker", marker, err)
		}
		if got := md.Serialize(); got != "- [ ] task\n" {
			t.Fatalf("SetCheckboxMarker(%q) changed the line: %q", marker, got)
		}
	}
	for _, marker := range []byte{'x', 'X', '~', ' '} {
		md := NewMDFileFromString("- [ ] task\n")
		if err := md.SetCheckboxMarker(1, marker); err != nil {
			t.Fatalf("SetCheckboxMarker(%q) = %v", marker, err)
		}
		if got, want := md.Serialize(), "- ["+string(marker)+"] task\n"; got != want {
			t.Fatalf("SetCheckboxMarker(%q) = %q, want %q", marker, got, want)
		}
	}
}
