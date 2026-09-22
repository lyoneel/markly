package markly

import (
	"strings"
	"testing"
)

// TestFindSectionBody covers the hosts portable-pool and notes section
// shapes plus heading boundary rules.
func TestFindSectionBody(t *testing.T) {
	doc := `---
title: Hosts
---

## Portable Pool

- device: reader
  note: desk
- device: drive

## Notes

Host notes live here.
More detail.
### Deeper keeps going
Still inside.

# Top Level Ends

after
`
	f := NewMDFileFromString(doc)

	t.Run("portable pool shape", func(t *testing.T) {
		body, found := f.FindSectionBody("portable-pool")
		if !found {
			t.Fatal("found = false, want true")
		}
		want := "- device: reader\n  note: desk\n- device: drive"
		if body != want {
			t.Errorf("body = %q, want %q", body, want)
		}
	})

	t.Run("notes shape runs until the next heading", func(t *testing.T) {
		body, found := f.FindSectionBody("notes")
		if !found {
			t.Fatal("found = false, want true")
		}
		if !strings.Contains(body, "Host notes live here.") {
			t.Errorf("body = %q, want the notes text", body)
		}
		if !strings.Contains(body, "### Deeper keeps going") {
			t.Errorf("body = %q, want the deeper heading inside the section", body)
		}
		if strings.Contains(body, "after") {
			t.Errorf("body = %q, want it to stop at the top-level heading", body)
		}
	})

	t.Run("missing slug reports not found", func(t *testing.T) {
		if _, found := f.FindSectionBody("missing"); found {
			t.Error("found = true, want false")
		}
	})

	t.Run("level-one heading is not a section target", func(t *testing.T) {
		if _, found := f.FindSectionBody("top-level-ends"); found {
			t.Error("found = true, want false (only ## sections resolve)")
		}
	})

	t.Run("trailing blank lines are trimmed", func(t *testing.T) {
		f := NewMDFileFromString("---\ntitle: T\n---\n\n## Notes\n\nline one\n\n\n## Next\n\nx\n")
		body, found := f.FindSectionBody("notes")
		if !found {
			t.Fatal("found = false, want true")
		}
		if body != "line one" {
			t.Errorf("body = %q, want %q", body, "line one")
		}
	})
}

// TestFindSectionBodyParityWithFindSection checks the two section APIs
// agree on the same slug set.
func TestFindSectionBodyParityWithFindSection(t *testing.T) {
	doc := "---\ntitle: T\n---\n\n## Notes\n\ntext\n"
	f := NewMDFileFromString(doc)
	_, foundLine := f.FindSection("notes")
	_, foundBody := f.FindSectionBody("notes")
	if foundLine != foundBody {
		t.Errorf("FindSection found=%v, FindSectionBody found=%v", foundLine, foundBody)
	}
}
