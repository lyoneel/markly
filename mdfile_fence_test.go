package markly

import "testing"

const fencedDoc = "---\ntitle: T\n---\n\n# Real Heading\n\n```\n# not a heading\n## also not\n```\n\n## Real Sub\n\ntext\n"

func TestDefaultHeadingDetectionIncludesFenced(t *testing.T) {
	md := NewMDFileFromString(fencedDoc)
	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	// Default: fenced lines count as headings (existing behavior).
	if len(c.Headings) != 4 {
		t.Errorf("default mode headings = %d, want 4", len(c.Headings))
	}
}

func TestFenceAwareHeadingDetection(t *testing.T) {
	md := NewMDFileFromString(fencedDoc, WithFenceAwareHeadings())
	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if len(c.Headings) != 2 {
		t.Fatalf("fence-aware headings = %d, want 2", len(c.Headings))
	}
	if c.Headings[0].Text != "Real Heading" || c.Headings[1].Text != "Real Sub" {
		t.Errorf("headings = %q, %q", c.Headings[0].Text, c.Headings[1].Text)
	}
	// Body content is untouched; only the heading list changes.
	c2, _ := NewMDFileFromString(fencedDoc).GetContent()
	if c.RawBody != c2.RawBody {
		t.Error("fence-aware mode must not alter the raw body")
	}
}

func TestFenceAwareTildeFences(t *testing.T) {
	doc := "# Top\n\n~~~\n# inside tilde\n~~~\n\n## Bottom\n"
	md := NewMDFileFromString(doc, WithFenceAwareHeadings())
	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if len(c.Headings) != 2 {
		t.Errorf("headings = %d, want 2 (Top, Bottom)", len(c.Headings))
	}
}
