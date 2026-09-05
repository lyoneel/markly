package markly

import (
	"os"
	"strings"
	"testing"
)

func TestMDFileWithoutYAMLMetadata(t *testing.T) {
	// Create a temporary Markdown file without YAML frontmatter
	tmpFile, err := os.CreateTemp("", "test-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content without YAML metadata
	content := `# Heading 1

Some text here.

## Subheading

More text under subheading.

### Another level

Even more nested content.
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test MDFile handling
	mdFile := NewMDFile(tmpFile.Name())

	// Metadata should be nil since there was no YAML frontmatter
	metadata, err := mdFile.GetMetadata()
	if err != nil {
		t.Errorf("GetMetadata returned unexpected error: %v", err)
	}
	if metadata != nil {
		t.Error("expected nil metadata for file without YAML frontmatter")
	}

	// LoadContent should still work and parse headings
	err = mdFile.LoadContent()
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	contentData, _ := mdFile.GetContent()
	if contentData == nil {
		t.Fatal("expected content to be loaded")
	}

	// Debug output
	t.Logf("RawBody length: %d", len(contentData.RawBody))
	t.Logf("Headings count: %d", len(contentData.Headings))
	for i, h := range contentData.Headings {
		t.Logf("Heading %d: level=%d text='%s' fromLine=%d toLine=%d", i, h.Level, h.Text, h.FromLine, h.ToLine)
	}

	// Verify headings were parsed correctly
	if len(contentData.Headings) != 3 {
		t.Errorf("expected 3 headings, got %d", len(contentData.Headings))
	}

	// Check first heading (only if we have at least one)
	if len(contentData.Headings) > 0 && contentData.Headings[0].Text != "Heading 1" {
		t.Errorf("expected 'Heading 1', got '%s'", contentData.Headings[0].Text)
	}

	// Verify line ranges for headings
	// Heading 1: "# Heading 1" is on line 1 (relative to content start)
	if len(contentData.Headings) > 0 {
		h := contentData.Headings[0]
		if h.FromLine != 1 || h.ToLine != 1 {
			t.Errorf("expected first heading FromLine=1, ToLine=1, got FromLine=%d, ToLine=%d", h.FromLine, h.ToLine)
		}
	}

	// Verify raw body contains the original content
	expectedBody := "# Heading 1\n\nSome text here.\n\n## Subheading\n\nMore text under subheading.\n\n### Another level\n\nEven more nested content."
	if contentData.RawBody != expectedBody {
		t.Errorf("raw body mismatch:\ngot:      %q\nexpected: %q", contentData.RawBody, expectedBody)
	}

	// Verify ContentFrom is 1 (no YAML to skip) - check after processMetadata()
	contentFrom := mdFile.GetContentFrom()
	if contentFrom != 1 {
		t.Errorf("expected ContentFrom=1, got %d", contentFrom)
	}
}

func TestMDFileWithYAMLMetadata(t *testing.T) {
	// Create a temporary markdown file WITH YAML frontmatter
	tmpFile, err := os.CreateTemp("", "test-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content with YAML metadata
	content := `---
title: Test Document
version: 1.0
tags:
  - test
  - markdown
author: John Doe
---

# Heading 1

Content after YAML frontmatter.

## Subheading

More content here.
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test MDFile handling
	mdFile := NewMDFile(tmpFile.Name())

	// Metadata should be populated
	metadata, err := mdFile.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata to be parsed")
	}

	// Verify metadata values
	if metadata.GetString("title") != "Test Document" {
		t.Errorf("expected title 'Test Document', got '%s'", metadata.GetString("title"))
	}

	version := metadata.Get("version")
	t.Logf("version raw value: %v (type: %T)", version, version)

	// YAML parses numbers as float64 by default, so "1.0" becomes 1.0 (float64)
	// GetString will return empty string for non-string values
	versionInt := metadata.GetInt("version")
	if versionInt != 1 {
		t.Errorf("expected version int 1, got %d", versionInt)
	}

	tags := metadata.GetStringList("tags")
	if len(tags) != 2 || tags[0] != "test" || tags[1] != "markdown" {
		t.Errorf("expected tags ['test', 'markdown'], got %v", tags)
	}

	if metadata.GetString("author") != "John Doe" {
		t.Errorf("expected author 'John Doe', got '%s'", metadata.GetString("author"))
	}

	// Verify metadata line range:
	// Line 1: --- (opening delimiter, not counted)
	// Lines 2-7: YAML content (6 lines)
	// Line 8: --- (closing delimiter, not counted)
	t.Logf("Metadata FromLine=%d ToLine=%d", metadata.FromLine, metadata.ToLine)
	if metadata.FromLine != 2 {
		t.Errorf("expected metadata FromLine=2, got %d", metadata.FromLine)
	}
	if metadata.ToLine != 7 {
		t.Errorf("expected metadata ToLine=7, got %d", metadata.ToLine)
	}

	// LoadContent should work and skip YAML frontmatter
	err = mdFile.LoadContent()
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	contentData, _ := mdFile.GetContent()
	if contentData == nil {
		t.Fatal("expected content to be loaded")
	}

	// Verify headings were parsed (should not include YAML section)
	if len(contentData.Headings) != 2 {
		t.Errorf("expected 2 headings, got %d", len(contentData.Headings))
	}

	// Debug: show what RawBody contains
	t.Logf("RawBody content:\n%s", contentData.RawBody)
	contentFrom := mdFile.GetContentFrom()
	t.Logf("ContentFrom: %d", contentFrom)

	// ContentFrom should be 9 (line after closing ---):
	// Line 1: --- (opening)
	// Lines 2-7: YAML content (6 lines)
	// Line 8: --- (closing)
	// Line 9+: Content starts here
	if contentFrom != 9 {
		t.Errorf("expected ContentFrom=9, got %d", contentFrom)
	}

	// Raw body should NOT contain the YAML frontmatter
	if len(contentData.RawBody) >= 3 && contentData.RawBody[:3] == "---" {
		t.Error("raw body should not contain YAML frontmatter")
	}
}

func TestMDFileEmptyFile(t *testing.T) {
	// Create an empty temporary file
	tmpFile, err := os.CreateTemp("", "test-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	mdFile := NewMDFile(tmpFile.Name())

	// processMetadata should handle empty file gracefully
	err = mdFile.processMetadata()
	if err != nil {
		t.Errorf("processMetadata returned unexpected error for empty file: %v", err)
	}

	// LoadContent should also handle empty file
	err = mdFile.LoadContent()
	if err != nil {
		t.Fatalf("LoadContent failed on empty file: %v", err)
	}

	contentData, _ := mdFile.GetContent()
	if contentData == nil {
		t.Fatal("expected content (even if empty)")
	}

	if len(contentData.Headings) != 0 {
		t.Errorf("expected no headings in empty file, got %d", len(contentData.Headings))
	}
}

func TestMDFileSave(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-save-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `---
title: Test Document
version: 1.0
tags:
  - test
  - markdown
author: John Doe
---

# Heading 1

Content after YAML frontmatter.

## Subheading

More content here.
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	mdFile := NewMDFile(tmpFile.Name())

	err = mdFile.processMetadata()
	if err != nil {
		t.Fatalf("processMetadata failed: %v", err)
	}

	err = mdFile.LoadContent()
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	err = mdFile.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	savedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if string(savedContent) == content {
		t.Log("Saved content matches original")
	} else {
		t.Logf("Original:\n%s\n\nSaved:\n%s", content, string(savedContent))
	}
}

func TestMDFileSaveWithSetType(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-save-type-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `---
title: Test Document
version: 1.0
tags:
  - test
  - markdown
author: John Doe
---

# Heading 1

Content after YAML frontmatter.
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	mdFile := NewMDFile(tmpFile.Name())

	err = mdFile.processMetadata()
	if err != nil {
		t.Fatalf("processMetadata failed: %v", err)
	}

	err = mdFile.LoadContent()
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	err = mdFile.SetType(FMTypeTOML)
	if err != nil {
		t.Fatalf("SetType failed: %v", err)
	}

	err = mdFile.Save()
	if err != nil {
		t.Fatalf("Save failed after SetType: %v", err)
	}

	savedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	savedStr := string(savedContent)
	if !strings.HasPrefix(savedStr, "+++") {
		t.Errorf("expected TOML format (starts with +++), got:\n%s", savedStr)
	}

	if strings.Contains(savedStr, "---") {
		t.Error("TOML format should not contain YAML delimiters")
	}
}

func TestMDFileSaveWithoutContent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-save-nocontent-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `---
title: Test Document
version: 1.0
---
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	mdFile := NewMDFile(tmpFile.Name())

	err = mdFile.processMetadata()
	if err != nil {
		t.Fatalf("processMetadata failed: %v", err)
	}

	err = mdFile.Save()
	if err != nil {
		t.Fatalf("Save failed for file without content: %v", err)
	}

	savedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if len(savedContent) == 0 {
		t.Error("saved file should not be empty")
	}
}

func TestMDFileSaveEmptyPath(t *testing.T) {
	mdFile := NewMDFile("")

	err := mdFile.Save()
	if err == nil {
		t.Fatal("expected error when saving with empty path, got nil")
	}

	if !strings.Contains(err.Error(), "cannot save: file path is empty") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestMDFileSaveInvalidType(t *testing.T) {
	mdFile := NewMDFile("/tmp/test.md")

	err := mdFile.SetType(FMType("invalid"))
	if err == nil {
		t.Fatal("expected error when setting invalid type, got nil")
	}

	expectedMsg := "unsupported frontmatter format: invalid (use 'yaml' or 'toml')"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}
