package markly

import (
	"os"
	"testing"
)

// TestMetadataLineRange tests the FromLine and ToLine tracking for metadata.
func TestMetadataLineRange(t *testing.T) {
	content := `---
name: test-doc
description: A doc with content
author: John Doe
version: 1.0
---

# Title

Some content here.`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md := NewMDFile(tmpFile)
	metadata, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}
	if metadata == nil {
		t.Fatal("Expected metadata but got nil")
	}

	// YAML content is from line 2 (after opening ---) to line 5 (before closing --- on line 6)
	expectedFromLine := 2
	expectedToLine := 5

	if metadata.FromLine != expectedFromLine {
		t.Errorf("Expected FromLine=%d, got %d", expectedFromLine, metadata.FromLine)
	}

	if metadata.ToLine != expectedToLine {
		t.Errorf("Expected ToLine=%d, got %d", expectedToLine, metadata.ToLine)
	}
}

// TestHeadingSingleLineSpan tests that a heading on its own line has FromLine == ToLine.
func TestHeadingSingleLineSpan(t *testing.T) {
	content := `---
name: test-doc
---

# Title`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)
	err := md.LoadContent()
	if err != nil {
		t.Fatalf("Failed to load content: %v", err)
	}

	contentData, err := md.GetContent()
	if err != nil || contentData == nil {
		t.Fatal("Failed to get content")
	}

	if len(contentData.Headings) != 1 {
		t.Fatalf("Expected 1 heading, got %d", len(contentData.Headings))
	}

	heading := contentData.Headings[0]
	if heading.FromLine != heading.ToLine {
		t.Errorf("Expected FromLine==ToLine for single-line heading, got FromLine=%d, ToLine=%d", heading.FromLine, heading.ToLine)
	}
}

// TestHeadingMultiLineSpan tests that a heading's ToLine extends to include content until next heading.
func TestHeadingMultiLineSpan(t *testing.T) {
	content := `---
name: test-doc
---

## Phase 1: Description
Description text here...
More description on line 2.
Even more on line 3.

## Phase 2`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)
	err := md.LoadContent()
	if err != nil {
		t.Fatalf("Failed to load content: %v", err)
	}

	contentData, err := md.GetContent()
	if err != nil || contentData == nil {
		t.Fatal("Failed to get content")
	}

	if len(contentData.Headings) != 2 {
		t.Fatalf("Expected 2 headings, got %d", len(contentData.Headings))
	}

	firstHeading := contentData.Headings[0]
	// First heading should span from its line until just before the second heading
	if firstHeading.ToLine <= firstHeading.FromLine {
		t.Errorf("Expected ToLine > FromLine for multi-line heading, got FromLine=%d, ToLine=%d", firstHeading.FromLine, firstHeading.ToLine)
	}
}
