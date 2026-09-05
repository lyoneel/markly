package markly

import (
	"strings"
	"testing"
)

// threeLineFM is a document with a 3-line frontmatter block and one
// blank separator line, so body line 0 is absolute file line 4 (the
// blank separator) and body line 1 is absolute file line 5.
const threeLineFM = "---\ntitle: T\n---\n\nl1\nl2\nl3\n"

func TestAppendInsertRemoveLines(t *testing.T) {
	md := NewMDFileFromString(threeLineFM)

	if err := md.AppendLine("l4"); err != nil {
		t.Fatalf("AppendLine failed: %v", err)
	}
	// Insert before absolute line 5 (body "l1").
	if err := md.InsertLine(5, "l0"); err != nil {
		t.Fatalf("InsertLine failed: %v", err)
	}
	if err := md.RemoveLine(7); err != nil { // removes original "l2"
		t.Fatalf("RemoveLine failed: %v", err)
	}

	// The leading blank separator line (body index 0) is preserved.
	body := md.content.RawBody
	want := "\nl0\nl1\nl3\nl4"
	if body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestInsertAppendAtEnd(t *testing.T) {
	md := NewMDFileFromString(threeLineFM)
	// len+1 inserts/appends at the end: body has 3 lines, frontmatter
	// occupies 4, so absolute line 8 is one past the last body line.
	if err := md.InsertLine(8, "end"); err != nil {
		t.Fatalf("InsertLine at end failed: %v", err)
	}
	if !strings.HasSuffix(md.content.RawBody, "end") {
		t.Errorf("body does not end with appended line: %q", md.content.RawBody)
	}
}

func TestLineOutOfRangeErrors(t *testing.T) {
	md := NewMDFileFromString(threeLineFM)

	if err := md.InsertLine(0, "x"); err == nil {
		t.Error("InsertLine(0) must fail")
	}
	if err := md.InsertLine(100, "x"); err == nil {
		t.Error("InsertLine(100) must fail")
	}
	if err := md.RemoveLine(1); err == nil {
		t.Error("RemoveLine into frontmatter must fail")
	}
	if err := md.RemoveLine(100); err == nil {
		t.Error("RemoveLine(100) must fail")
	}
}

func TestSetBody(t *testing.T) {
	md := NewMDFileFromString(threeLineFM)
	md.SetBody("fresh\nbody")
	if md.content.RawBody != "fresh\nbody" {
		t.Errorf("RawBody = %q", md.content.RawBody)
	}
	out := md.Serialize()
	if !strings.Contains(out, "fresh") {
		t.Errorf("serialized output missing new body:\n%s", out)
	}
}

func TestExtractRemoveFirstH1(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: T\n---\n\n# My Title\n\nbody\n")

	title, found := md.ExtractFirstH1()
	if !found || title != "My Title" {
		t.Errorf("ExtractFirstH1 = %q, %v; want My Title, true", title, found)
	}

	if err := md.RemoveFirstH1(); err != nil {
		t.Fatalf("RemoveFirstH1 failed: %v", err)
	}
	if strings.Contains(md.content.RawBody, "# My Title") {
		t.Errorf("H1 still present after removal: %q", md.content.RawBody)
	}

	_, found = md.ExtractFirstH1()
	if found {
		t.Error("no H1 should remain")
	}
}

func TestFirstH1InsideFenceIgnored(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: T\n---\n\n```\n# not a title\n```\n\nbody\n")

	_, found := md.ExtractFirstH1()
	if found {
		t.Error("H1 inside code fence must not be found")
	}
}

func TestFirstH1TildeFenceIgnored(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: T\n---\n\n~~~\n# not a title\n~~~\n\n# Real Title\n")

	title, found := md.ExtractFirstH1()
	if !found || title != "Real Title" {
		t.Errorf("ExtractFirstH1 = %q, %v; want Real Title", title, found)
	}
}

func TestSetFirstHeadingReplaceAndInsert(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: T\n---\n\n# Old\n\nbody\n")
	if err := md.SetFirstHeading("New"); err != nil {
		t.Fatalf("SetFirstHeading failed: %v", err)
	}
	if !strings.Contains(md.content.RawBody, "# New") || strings.Contains(md.content.RawBody, "# Old") {
		t.Errorf("heading not replaced: %q", md.content.RawBody)
	}

	md2 := NewMDFileFromString("---\ntitle: T\n---\n\nbody only\n")
	if err := md2.SetFirstHeading("Inserted"); err != nil {
		t.Fatalf("SetFirstHeading insert failed: %v", err)
	}
	if !strings.HasPrefix(md2.content.RawBody, "# Inserted") {
		t.Errorf("heading not inserted at top: %q", md2.content.RawBody)
	}
}

func TestSlugHeading(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"  Spaced   Out  ", "spaced-out"},
		{"Punctuation! Here?", "punctuation-here"},
		{"already-slug", "already-slug"},
		{"---leading and trailing---", "leading-and-trailing"},
		{"UPPER lower 123", "upper-lower-123"},
	}
	for _, c := range cases {
		if got := SlugHeading(c.in); got != c.want {
			t.Errorf("SlugHeading(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
