package markly

import (
	"strings"
	"testing"
)

const sectionDoc = "---\ntitle: T\n---\n\n## Section One\n- status: next\n\n- [ ] a task\n\n## Second\n\ntext\n"

func TestFindSection(t *testing.T) {
	md := NewMDFileFromString(sectionDoc)

	line, found := md.FindSection("section-one")
	if !found {
		t.Fatal("section-one not found")
	}
	// Frontmatter occupies lines 1-3, blank separator line 4, heading on line 5.
	if line != 5 {
		t.Errorf("FindSection line = %d, want 5", line)
	}

	if _, found := md.FindSection("missing"); found {
		t.Error("missing section must not be found")
	}
}

func TestSetSectionHeading(t *testing.T) {
	md := NewMDFileFromString(sectionDoc)

	if err := md.SetSectionHeading("section-one", "Renamed"); err != nil {
		t.Fatalf("SetSectionHeading failed: %v", err)
	}
	if !strings.Contains(md.content.RawBody, "## Renamed") {
		t.Errorf("heading not renamed: %q", md.content.RawBody)
	}
	if _, found := md.FindSection("renamed"); !found {
		t.Error("renamed section not found by new slug")
	}

	if err := md.SetSectionHeading("missing", "X"); err == nil {
		t.Error("renaming a missing section must fail")
	}
}

func TestSetSectionBulletReplaceAndInsert(t *testing.T) {
	md := NewMDFileFromString(sectionDoc)

	// Replace existing "- status: next".
	if err := md.SetSectionBullet("section-one", "status", "done"); err != nil {
		t.Fatalf("SetSectionBullet replace failed: %v", err)
	}
	if !strings.Contains(md.content.RawBody, "- status: done") {
		t.Errorf("bullet not replaced: %q", md.content.RawBody)
	}
	if strings.Contains(md.content.RawBody, "- status: next") {
		t.Error("old bullet still present")
	}

	// Insert new key under Second (no bullets yet).
	if err := md.SetSectionBullet("second", "due", "2026-08-14"); err != nil {
		t.Fatalf("SetSectionBullet insert failed: %v", err)
	}
	idx := strings.Index(md.content.RawBody, "## Second")
	bulletIdx := strings.Index(md.content.RawBody, "- due: 2026-08-14")
	if idx < 0 || bulletIdx < 0 || bulletIdx < idx {
		t.Errorf("inserted bullet misplaced: %q", md.content.RawBody)
	}
}

func TestInsertLineUnderHeading(t *testing.T) {
	md := NewMDFileFromString(sectionDoc)

	// Insert under an existing heading: must land after the "- status:"
	// metadata bullet.
	if err := md.InsertLineUnderHeading("Section One", "- [ ] moved task", false); err != nil {
		t.Fatalf("InsertLineUnderHeading failed: %v", err)
	}
	statusIdx := strings.Index(md.content.RawBody, "- status: next")
	movedIdx := strings.Index(md.content.RawBody, "- [ ] moved task")
	if statusIdx < 0 || movedIdx < 0 || movedIdx < statusIdx {
		t.Errorf("inserted line not after metadata bullet: %q", md.content.RawBody)
	}

	// Missing heading without creation fails.
	if err := md.InsertLineUnderHeading("Nope", "- x", false); err == nil {
		t.Error("insert under missing heading must fail without createMissing")
	}

	// Missing heading with creation appends at the end.
	if err := md.InsertLineUnderHeading("Fresh Section", "- [ ] fresh", true); err != nil {
		t.Fatalf("InsertLineUnderHeading create failed: %v", err)
	}
	if !strings.HasSuffix(md.content.RawBody, "## Fresh Section\n- [ ] fresh") {
		t.Errorf("created section not appended at end: %q", md.content.RawBody)
	}
}

// checkboxDoc: frontmatter lines 1-3, blank line 4, body starts line 5.
const checkboxDoc = "---\ntitle: T\n---\n\n- [ ] open task (2026-08-13 10:00)\n- [x] done task (due: 2026-08-20)\n- not a checkbox\n"

func TestSetCheckboxMarker(t *testing.T) {
	md := NewMDFileFromString(checkboxDoc)

	if err := md.SetCheckboxMarker(5, 'x'); err != nil {
		t.Fatalf("SetCheckboxMarker failed: %v", err)
	}
	if !strings.Contains(md.content.RawBody, "- [x] open task (2026-08-13 10:00)") {
		t.Errorf("marker not flipped: %q", md.content.RawBody)
	}

	if err := md.SetCheckboxMarker(6, ' '); err != nil {
		t.Fatalf("SetCheckboxMarker reopen failed: %v", err)
	}
	if !strings.Contains(md.content.RawBody, "- [ ] done task (due: 2026-08-20)") {
		t.Errorf("marker not reopened: %q", md.content.RawBody)
	}

	if err := md.SetCheckboxMarker(7, 'x'); err == nil {
		t.Error("non-checkbox line must fail")
	}
	if err := md.SetCheckboxMarker(99, 'x'); err == nil {
		t.Error("out-of-range line must fail")
	}
}

func TestRewriteCheckboxTitle(t *testing.T) {
	md := NewMDFileFromString(checkboxDoc)

	if err := md.RewriteCheckboxTitle(5, "renamed task"); err != nil {
		t.Fatalf("RewriteCheckboxTitle failed: %v", err)
	}
	if !strings.Contains(md.content.RawBody, "- [ ] renamed task (2026-08-13 10:00)") {
		t.Errorf("title not rewritten with suffix preserved: %q", md.content.RawBody)
	}
	if strings.Contains(md.content.RawBody, "open task") {
		t.Error("old title still present")
	}

	md2 := NewMDFileFromString(checkboxDoc)
	if err := md2.RewriteCheckboxTitle(6, "changed"); err != nil {
		t.Fatalf("RewriteCheckboxTitle due suffix failed: %v", err)
	}
	if !strings.Contains(md2.content.RawBody, "- [x] changed (due: 2026-08-20)") {
		t.Errorf("due suffix not preserved: %q", md2.content.RawBody)
	}

	if err := md2.RewriteCheckboxTitle(7, "x"); err == nil {
		t.Error("non-checkbox line must fail")
	}
}

func TestIsCaptureTimestamp(t *testing.T) {
	if !isCaptureTimestamp("2026-08-13 10:00") {
		t.Error("valid timestamp rejected")
	}
	if isCaptureTimestamp("2026-08-13T10:00") {
		t.Error("invalid separator accepted")
	}
	if isCaptureTimestamp("not a timestamp") {
		t.Error("garbage accepted")
	}
}
