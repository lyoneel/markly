package markly

import (
	"os"
	"testing"
)

// TestContentExtraction tests content extraction from markdown files.
func TestContentExtraction(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectHeadings int
		checkRawBody   func(string, error) bool
	}{
		{
			name: "File with no frontmatter",
			content: `# Title

Some content here.`,
			expectHeadings: 1,
			checkRawBody: func(body string, err error) bool {
				if err != nil || body == "" {
					return false
				}
				return len(body) > 0 && body[0] == '#'
			},
		},
		{
			name: "Frontmatter only (no body)",
			content: `---
name: test-doc
description: A doc without body
---`,
			expectHeadings: 0,
			checkRawBody: func(body string, err error) bool {
				if err != nil {
					return false
				}
				return body == "" || len(body) == 0
			},
		},
		{
			name: "Multiple consecutive headings",
			content: `---
name: test
---

## Section
### Subsection
#### Detail`,
			expectHeadings: 3,
			checkRawBody: func(body string, err error) bool {
				if err != nil || body == "" {
					return false
				}
				return true // Just check it has content
			},
		},
		{
			name: "Heading followed by content",
			content: `---
name: test
---

## Phase 1: Description
Description text here...
More description.`,
			expectHeadings: 1,
			checkRawBody: func(body string, err error) bool {
				if err != nil || body == "" {
					return false
				}
				return true // Just check it has content
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md, _ := NewMDFileWithContent(tmpFile)
			err := md.LoadContent()
			content, err := md.GetContent()

			if content == nil && err == nil {
				t.Errorf("Expected content or error but got neither")
				return
			}

			if len(content.Headings) != tt.expectHeadings {
				t.Errorf("Expected %d headings, got %d", tt.expectHeadings, len(content.Headings))
				return
			}

			if !tt.checkRawBody(content.RawBody, err) {
				t.Errorf("Raw body check failed for test: %s", tt.name)
			}
		})
	}
}

// TestContentFromLine tests the contentFrom line number tracking.
func TestContentFromLine(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		expectContentFrom int
	}{
		{
			name: "No frontmatter - content starts at 1",
			content: `# Title

Content here.`,
			expectContentFrom: 1,
		},
		{
			name: "With frontmatter - content starts after closing ---",
			content: `---
name: test-doc
description: A doc
---

# Title

Content here.`,
			expectContentFrom: 5, // Line 5 is "# Title" (after closing --- on line 4)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md, _ := NewMDFileWithContent(tmpFile)
			err := md.LoadContent()

			if err != nil {
				t.Errorf("Failed to load content: %v", err)
				return
			}

			contentFrom := md.GetContentFrom()
			if contentFrom != tt.expectContentFrom {
				t.Errorf("Expected contentFrom=%d, got %d", tt.expectContentFrom, contentFrom)
			}
		})
	}
}
