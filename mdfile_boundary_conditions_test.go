package markly

import (
	"os"
	"testing"
)

// TestMaximumHeadingLevel tests that level 6 is the maximum valid heading.
func TestMaximumHeadingLevel(t *testing.T) {
	content := `---
name: test-doc
---

###### Deepest`

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

	if contentData.Headings[0].Level != 6 {
		t.Errorf("Expected level=6, got %d", contentData.Headings[0].Level)
	}
}

// TestMinimumValidFrontmatter tests the minimum valid frontmatter (single field).
func TestMinimumValidFrontmatter(t *testing.T) {
	content := `---
name: x
---

# Content`

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

	if metadata.GetString("name") != "x" {
		t.Errorf("Expected name='x', got '%s'", metadata.GetString("name"))
	}
}

// TestVeryLongHeadingText tests that very long heading text is preserved.
func TestVeryLongHeadingText(t *testing.T) {
	longText := "This is an extremely long heading that spans many words and should be fully preserved in the parsed output without any truncation or modification whatsoever"
	content := `---
name: test-doc
---

# ` + longText

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

	if contentData.Headings[0].Text != longText {
		t.Errorf("Heading text was not preserved correctly. Expected length=%d, got length=%d", len(longText), len(contentData.Headings[0].Text))
	}
}

// TestEmptyFile tests handling of completely empty file.
func TestEmptyFile(t *testing.T) {
	content := ``

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)
	err := md.LoadContent()
	if err != nil {
		t.Fatalf("Failed to load empty file: %v", err)
	}

	contentData, err := md.GetContent()
	if err != nil || contentData == nil {
		t.Fatal("Failed to get content from empty file")
	}

	if len(contentData.Headings) != 0 {
		t.Errorf("Expected 0 headings in empty file, got %d", len(contentData.Headings))
	}

	if contentData.RawBody != "" {
		t.Errorf("Expected empty RawBody, got '%s'", contentData.RawBody)
	}
}

// TestOnlyWhitespaceFile tests handling of file with only whitespace.
func TestOnlyWhitespaceFile(t *testing.T) {
	content := `   
	
	`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)
	err := md.LoadContent()
	if err != nil {
		t.Fatalf("Failed to load whitespace-only file: %v", err)
	}

	contentData, err := md.GetContent()
	if err != nil || contentData == nil {
		t.Fatal("Failed to get content from whitespace-only file")
	}

	if len(contentData.Headings) != 0 {
		t.Errorf("Expected 0 headings in whitespace-only file, got %d", len(contentData.Headings))
	}
}

// TestOnlyFrontmatterNoContent tests handling of file with only frontmatter and no body.
func TestOnlyFrontmatterNoContent(t *testing.T) {
	content := `---
name: test-doc
description: A doc without any body content
author: John Doe
version: 1.0
---`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)
	err := md.LoadContent()
	if err != nil {
		t.Fatalf("Failed to load frontmatter-only file: %v", err)
	}

	contentData, err := md.GetContent()
	if err != nil || contentData == nil {
		t.Fatal("Failed to get content from frontmatter-only file")
	}

	if len(contentData.Headings) != 0 {
		t.Errorf("Expected 0 headings in frontmatter-only file, got %d", len(contentData.Headings))
	}
}

// TestMultipleEmptyLinesBetweenHeadings tests handling of multiple empty lines between headings.
func TestMultipleEmptyLinesBetweenHeadings(t *testing.T) {
	content := `---
name: test-doc
---

# First Heading


## Second Heading



### Third Heading`

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

	if len(contentData.Headings) != 3 {
		t.Errorf("Expected 3 headings, got %d", len(contentData.Headings))
		return
	}

	expectedLevels := []int{1, 2, 3}
	for i, h := range contentData.Headings {
		if h.Level != expectedLevels[i] {
			t.Errorf("Heading[%d]: Expected level=%d, got %d", i, expectedLevels[i], h.Level)
		}
	}
}
