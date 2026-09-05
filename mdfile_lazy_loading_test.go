package markly

import (
	"os"
	"testing"
)

// TestLazyLoading tests the lazy loading functionality.
func TestLazyLoading(t *testing.T) {
	content := `---
name: test-doc
description: A doc with content
---

# Title

Some content here.`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	tests := []struct {
		name        string
		setup       func(*MDFile)
		expectError bool
		checkResult func(*MDContent, error) bool
	}{
		{
			name: "Lazy loading enabled (default)",
			setup: func(md *MDFile) {
				md.SetLazyLoading(true)
			},
			expectError: true,
			checkResult: func(content *MDContent, err error) bool {
				return content == nil && err != nil && err.Error() == "content not yet loaded; call LoadContent() first"
			},
		},
		{
			name: "Eager loading disabled",
			setup: func(md *MDFile) {
				md.SetLazyLoading(false)
			},
			expectError: false,
			checkResult: func(content *MDContent, err error) bool {
				return content != nil && err == nil && len(content.Headings) == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := NewMDFile(tmpFile)
			tt.setup(md)

			content, err := md.GetContent()

			if !tt.checkResult(content, err) {
				t.Errorf("Lazy loading check failed for test: %s", tt.name)
			}
		})
	}
}

// TestDoubleLoadAttempt tests that calling LoadContent twice doesn't cause issues.
func TestDoubleLoadAttempt(t *testing.T) {
	content := `---
name: test-doc
description: A doc with content
---

# Title

Some content here.`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)

	// First load
	err1 := md.LoadContent()
	if err1 != nil {
		t.Errorf("First LoadContent failed: %v", err1)
		return
	}

	content1, err1 := md.GetContent()
	if content1 == nil || err1 != nil {
		t.Errorf("Failed to get content after first load")
		return
	}

	// Second load (should be a no-op)
	err2 := md.LoadContent()
	if err2 != nil {
		t.Errorf("Second LoadContent failed: %v", err2)
		return
	}

	content2, err2 := md.GetContent()
	if content2 == nil || err2 != nil {
		t.Errorf("Failed to get content after second load")
		return
	}

	// Both should be the same (same pointer)
	if content1 != content2 {
		t.Errorf("Content pointers differ between loads")
	}
}
