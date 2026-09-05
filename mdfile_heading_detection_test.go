package markly

import (
	"os"
	"testing"
)

// TestHeadingDetection tests ATX-style heading detection.
func TestHeadingDetection(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectHeadings int
		checkHeadings  func([]*MDHeading, error) bool
	}{
		{
			name: "ATX heading level 1-6",
			content: `---
name: test
---

# Title
## Section
### Subsection
#### Detail
##### Minor
###### Deepest`,
			expectHeadings: 6,
			checkHeadings: func(headings []*MDHeading, err error) bool {
				if err != nil || len(headings) != 6 {
					return false
				}
				expectedLevels := []int{1, 2, 3, 4, 5, 6}
				for i, h := range headings {
					if h.Level != expectedLevels[i] {
						return false
					}
				}
				return true
			},
		},
		{
			name: "Heading with trailing markers",
			content: `---
name: test
---

# Title ###`,
			expectHeadings: 1,
			checkHeadings: func(headings []*MDHeading, err error) bool {
				if err != nil || len(headings) != 1 {
					return false
				}
				return headings[0].Level == 1 && headings[0].Text == "Title ###"
			},
		},
		{
			name: "Invalid heading (>6 hashes)",
			content: `---
name: test
---

####### Too deep`,
			expectHeadings: 0,
			checkHeadings: func(headings []*MDHeading, err error) bool {
				if err != nil || len(headings) != 0 {
					return false
				}
				return true
			},
		},
		{
			name: "Hash in middle of line",
			content: `---
name: test
---

Code: #include <stdio.h>`,
			expectHeadings: 0,
			checkHeadings: func(headings []*MDHeading, err error) bool {
				if err != nil || len(headings) != 0 {
					return false
				}
				return true
			},
		},
		{
			name: "Heading with no text",
			content: `---
name: test
---

###`,
			expectHeadings: 1,
			checkHeadings: func(headings []*MDHeading, err error) bool {
				if err != nil || len(headings) != 1 {
					return false
				}
				return headings[0].Level == 3 && headings[0].Text == ""
			},
		},
		{
			name: "Mixed whitespace before heading",
			content: `---
name: test
---

   ## Section`,
			expectHeadings: 1,
			checkHeadings: func(headings []*MDHeading, err error) bool {
				if err != nil || len(headings) != 1 {
					return false
				}
				return headings[0].Level == 2 && headings[0].Text == "Section"
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

			if !tt.checkHeadings(content.Headings, err) {
				t.Errorf("Heading check failed for test: %s", tt.name)
			}
		})
	}
}

// TestHeadingTextExtraction tests text extraction from headings.
func TestHeadingTextExtraction(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedText string
	}{
		{
			name: "Simple heading",
			content: `---
name: test
---

# Simple Title`,
			expectedText: "Simple Title",
		},
		{
			name: "Heading with inline formatting",
			content: `---
name: test
---

# Heading with **bold** and *italic*`,
			expectedText: "Heading with **bold** and *italic*",
		},
		{
			name: "Heading with link",
			content: `---
name: test
---

# [Link Text](https://example.com)`,
			expectedText: "[Link Text](https://example.com)",
		},
		{
			name:         "Heading with code span",
			content:      "---\nname: test\n---\n\n# Use `command` to run",
			expectedText: "Use `command` to run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md, _ := NewMDFileWithContent(tmpFile)
			err := md.LoadContent()
			content, err := md.GetContent()

			if err != nil || content == nil || len(content.Headings) != 1 {
				t.Errorf("Failed to load heading: %v", err)
				return
			}

			if content.Headings[0].Text != tt.expectedText {
				t.Errorf("Expected text '%s', got '%s'", tt.expectedText, content.Headings[0].Text)
			}
		})
	}
}
