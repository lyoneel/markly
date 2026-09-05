package markly

import (
	"os"
	"testing"
)

// TestYAMLFrontmatterParsing tests basic YAML frontmatter parsing scenarios.
func TestYAMLFrontmatterParsing(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectError   bool
		checkMetadata func(*MDMetadata, error) bool
		checkType     func(FMType) bool
	}{
		{
			name:    "Empty frontmatter (just ---)",
			content: "---\n",
			checkMetadata: func(m *MDMetadata, err error) bool {
				return m == nil && err == nil // No metadata when empty
			},
		},
		{
			name: "Minimal valid YAML",
			content: `---
name: test-doc
description: A doc
---

# Title`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test-doc" && m.GetString("description") == "A doc"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name: "Nested maps in YAML",
			content: `---
sources:
  spec: https://example.com
  docs: ../docs/README.md
---

# Content`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				sources := m.GetMap("sources")
				if sources == nil {
					return false
				}
				return sources["spec"] == "https://example.com" && sources["docs"] == "../docs/README.md"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name: "Mixed types (string, int, bool)",
			content: `---
name: test
count: 42
enabled: true
---

# Content`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test" && m.GetInt("count") == 42 && m.GetBool("enabled") == true
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name: "YAML arrays/lists",
			content: `---
tags:
  - feature
  - critical
---

# Content`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				tags := m.GetStringList("tags")
				if tags == nil || len(tags) != 2 {
					return false
				}
				return tags[0] == "feature" && tags[1] == "critical"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name: "Float64 parsed as int",
			content: `---
timeout: 30.5
---

# Content`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetInt("timeout") == 30 // Truncated from 30.5
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md := NewMDFile(tmpFile)
			meta, err := md.GetMetadata()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tt.checkMetadata(meta, err) {
				t.Errorf("Metadata check failed for test case: %s", tt.name)
			}

			// Check type field if provided
			if md.Type != "" && tt.checkType != nil && !tt.checkType(md.Type) {
				t.Errorf("Type check failed for test case: %s, got %q", tt.name, string(md.Type))
			}
		})
	}
}

// TestYAMLParsingEdgeCases tests edge cases for YAML parsing.
func TestYAMLParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		checkResult func(*MDMetadata, error) bool
		checkType   func(FMType) bool
	}{
		{
			name: "Invalid YAML syntax",
			content: `---
name: test
  invalid_indent: value
---

# Content`,
			expectError: true,
			checkResult: func(m *MDMetadata, err error) bool {
				return err != nil && m == nil
			},
			checkType: func(t FMType) bool { return t == "" }, // No type when error
		},
		{
			name: "Missing closing delimiter",
			content: `---
name: test-doc
description: A doc without closing

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test-doc" && m.GetString("description") == "A doc without closing"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name:        "Multiple --- in file (code blocks)",
			content:     "---\nname: test-doc\n---\n\n# Content\n\n```yaml\n---\nthis is not frontmatter\n---\n```\n",
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test-doc"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name: "Empty lines within YAML",
			content: `---
name: test

description: doc with empty line above
---

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test" && m.GetString("description") == "doc with empty line above"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
		{
			name: "Special characters in values",
			content: `---
description: "Use when creating docs"
command: echo 'hello world'
---

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("description") == "Use when creating docs" && m.GetString("command") == "echo 'hello world'"
			},
			checkType: func(t FMType) bool { return t == "yaml" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md := NewMDFile(tmpFile)
			meta, err := md.GetMetadata()

			if !tt.checkResult(meta, err) {
				t.Errorf("Edge case check failed for test: %s", tt.name)
			}

			// Check type field if provided
			if md.Type != "" && tt.checkType != nil && !tt.checkType(md.Type) {
				t.Errorf("Type check failed for test: %s, got %q", tt.name, md.Type)
			}
		})
	}
}

// Helper function to create a temporary file with content.
func createTempFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "markly-test-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write content to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

// TestTOMLFrontmatterParsing tests basic TOML frontmatter parsing scenarios.
func TestTOMLFrontmatterParsing(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectError   bool
		checkMetadata func(*MDMetadata, error) bool
		checkType     func(FMType) bool
	}{
		{
			name: "Basic TOML frontmatter",
			content: `+++
name = "test-doc"
description = "A doc with TOML"
+++

# Title`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test-doc" && m.GetString("description") == "A doc with TOML"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name: "TOML nested tables",
			content: `+++
[sources]
spec = "https://example.com"
docs = "../docs/README.md"
+++

# Content`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				sources := m.GetMap("sources")
				if sources == nil {
					return false
				}
				return sources["spec"] == "https://example.com" && sources["docs"] == "../docs/README.md"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name: "TOML arrays and mixed types",
			content: `+++
tags = ["feature", "critical"]
count = 42
enabled = true
timeout = 30.5
+++

# Content`,
			checkMetadata: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				tags := m.GetStringList("tags")
				if tags == nil || len(tags) != 2 {
					return false
				}
				return tags[0] == "feature" && tags[1] == "critical" &&
					m.GetInt("count") == 42 &&
					m.GetBool("enabled") == true &&
					m.GetInt("timeout") == 30 // Truncated from 30.5 (float64)
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md := NewMDFile(tmpFile)
			meta, err := md.GetMetadata()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tt.checkMetadata(meta, err) {
				t.Errorf("TOML metadata check failed for test case: %s", tt.name)
			}

			// Check type field if provided
			if md.Type != "" && tt.checkType != nil && !tt.checkType(md.Type) {
				t.Errorf("Type check failed for TOML test case: %s, got %q", tt.name, md.Type)
			}
		})
	}
}

// TestTOMLLineNumberTracking tests that line numbers are correctly tracked for TOML frontmatter.
func TestTOMLLineNumberTracking(t *testing.T) {
	content := `+++
name = "test"
description = "doc with empty line above"
active = true
+++

# Content`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md := NewMDFile(tmpFile)
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if meta == nil {
		t.Fatal("Expected metadata to be parsed")
	}

	// TOML starts at line 2 (after opening +++), ends at line 4 (before closing +++)
	if meta.FromLine != 2 {
		t.Errorf("FromLine = %d, want 2", meta.FromLine)
	}
	if meta.ToLine != 4 {
		t.Errorf("ToLine = %d, want 4", meta.ToLine)
	}

	// Verify type is set correctly
	if md.Type != "toml" {
		t.Errorf("Type = %q, want toml", md.Type)
	}
	if meta.GetType() != "toml" {
		t.Errorf("Metadata Type = %q, want toml", meta.GetType())
	}
}

// TestDelimiterDetection tests that correct parser is selected based on delimiter.
func TestDelimiterDetection(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		expectYAML bool // true = YAML parser, false = TOML parser
	}{
		{"YAML with ---", "---\nkey: value\n---\n\n# H1", true},
		{"TOML with +++", "+++\nkey = \"value\"\n+++\n\n# H1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md := NewMDFile(tmpFile)

			// Test metadata extraction
			meta, err := md.GetMetadata()
			if err != nil {
				t.Fatalf("GetMetadata failed: %v", err)
			}

			if meta == nil {
				t.Fatal("Expected metadata to be parsed")
			}

			// Verify data was extracted (format-agnostic check)
			value := meta.GetString("key")
			if value != "value" {
				t.Errorf("Failed to extract key: got %q, want %q", value, "value")
			}

			// Verify type is set correctly
			expectedType := "yaml"
			if !tt.expectYAML {
				expectedType = "toml"
			}
			if md.Type != FMType(expectedType) {
				t.Errorf("MDFile Type = %q, want %s", string(md.Type), expectedType)
			}
			if meta.GetType() != expectedType {
				t.Errorf("Metadata GetType() = %q, want %s", meta.GetType(), expectedType)
			}
		})
	}
}

// TestMixedDelimiterInCodeBlock tests that delimiters in code blocks are ignored.
func TestMixedDelimiterInCodeBlock(t *testing.T) {
	content := `+++
name = "test-doc"
+++

# Content

` + "```yaml\n---\nthis is not frontmatter\n+++\nalso not frontmatter\n```\n"

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md := NewMDFile(tmpFile)
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if meta == nil {
		t.Fatal("Expected metadata to be parsed")
	}

	if meta.GetString("name") != "test-doc" {
		t.Errorf("Failed to extract name from TOML frontmatter")
	}

	// Verify type is set correctly
	if md.Type != "toml" {
		t.Errorf("Type = %q, want toml", md.Type)
	}
}

// TestTOMLParsingEdgeCases tests edge cases for TOML parsing.
func TestTOMLParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		checkResult func(*MDMetadata, error) bool
		checkType   func(FMType) bool
	}{
		{
			name: "Invalid TOML syntax (unclosed table)",
			content: `+++
[section
key = "value"
+++

# Content`,
			expectError: true,
			checkResult: func(m *MDMetadata, err error) bool {
				return err != nil && m == nil
			},
			checkType: func(t FMType) bool { return t == "" }, // No type when error
		},
		{
			name: "Missing closing delimiter",
			content: `+++
name = "test-doc"
description = "A doc without closing"

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test-doc" && m.GetString("description") == "A doc without closing"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name:        "Multiple +++ in file (code blocks)",
			content:     "+++\nname = \"test-doc\"\n+++\n\n# Content\n\n```toml\n+++\nthis is not frontmatter\n+++\n```\n",
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test-doc"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name: "Empty lines within TOML",
			content: `+++
name = "test"

description = "doc with empty line above"
+++

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("name") == "test" && m.GetString("description") == "doc with empty line above"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name: "Special characters in TOML strings",
			content: `+++
description = "Use when creating docs"
command = "echo 'hello world'"
path = "C:\\Users\\test"
+++

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				return m.GetString("description") == "Use when creating docs" &&
					m.GetString("command") == "echo 'hello world'" &&
					m.GetString("path") == "C:\\Users\\test"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name: "TOML inline tables",
			content: `+++
point = { x = 1, y = 2 }
+++

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				point := m.GetMap("point")
				if point == nil {
					return false
				}
				return point["x"] == int64(1) && point["y"] == int64(2)
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
		{
			name: "TOML arrays of tables",
			content: `+++
[[items]]
name = "first"

[[items]]
name = "second"
+++

# Content`,
			expectError: false,
			checkResult: func(m *MDMetadata, err error) bool {
				if err != nil || m == nil {
					return false
				}
				items := m.GetList("items")
				if items == nil || len(items) != 2 {
					return false
				}
				first := items[0].(map[string]any)
				second := items[1].(map[string]any)
				return first["name"] == "first" && second["name"] == "second"
			},
			checkType: func(t FMType) bool { return t == "toml" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			md := NewMDFile(tmpFile)
			meta, err := md.GetMetadata()

			if !tt.checkResult(meta, err) {
				t.Errorf("TOML edge case check failed for test: %s", tt.name)
			}

			// Check type field if provided
			if md.Type != "" && tt.checkType != nil && !tt.checkType(md.Type) {
				t.Errorf("Type check failed for TOML edge case: %s, got %q", tt.name, md.Type)
			}
		})
	}
}
