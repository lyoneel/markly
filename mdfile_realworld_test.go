package markly

import (
	"os"
	"testing"
)

// TestRealWorldComplexNestedKaizenStructure tests parsing of complex nested kaizen structures.
func TestRealWorldComplexNestedKaizenStructure(t *testing.T) {
	content := `---
name: test-skill
description: A skill with complex metadata
recommended: true
kaizen:
  sources:
    spec-doc: https://example.com/specification
    docs-ref: ../docs/README.md
  targets:
    spec-doc: update specification limits and fields
    docs-ref: update documentation patterns
  extra:
    - verify all hard limits are current
    - maintain compatibility accuracy
---

# Test Skill Title

This is a test skill description.`

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

	// Test basic fields
	if metadata.GetString("name") != "test-skill" {
		t.Errorf("Expected name='test-skill', got '%s'", metadata.GetString("name"))
	}

	if !metadata.GetBool("recommended") {
		t.Error("Expected recommended=true")
	}

	// Test nested kaizen structure
	kaizen := metadata.GetMap("kaizen")
	if kaizen == nil {
		t.Fatal("Expected kaizen map but got nil")
	}

	sources := kaizen["sources"].(map[string]any)
	if sources["spec-doc"] != "https://example.com/specification" {
		t.Errorf("Expected spec-doc URL, got '%v'", sources["spec-doc"])
	}

	targets := kaizen["targets"].(map[string]any)
	if targets["spec-doc"] != "update specification limits and fields" {
		t.Errorf("Expected spec-doc target description")
	}

	extra := kaizen["extra"].([]any)
	if len(extra) != 2 {
		t.Errorf("Expected 2 extra items, got %d", len(extra))
	}
}

// TestRealWorldBooleanFrontmatterFields tests parsing of boolean frontmatter fields.
func TestRealWorldBooleanFrontmatterFields(t *testing.T) {
	content := `---
name: test-skill
description: A skill with booleans
user-invocable: true
disable-model-invocation: false
recommended: true
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

	if !metadata.GetBool("user-invocable") {
		t.Error("Expected user-invocable=true")
	}

	if metadata.GetBool("disable-model-invocation") {
		t.Error("Expected disable-model-invocation=false")
	}

	if !metadata.GetBool("recommended") {
		t.Error("Expected recommended=true")
	}
}

// TestRealWorldMixedArrayTypes tests parsing of mixed array types in frontmatter.
func TestRealWorldMixedArrayTypes(t *testing.T) {
	content := `---
name: test-skill
description: A skill with arrays
workflows:
  - generate
  - validate
  - execute
required-frontmatter:
  - name
  - purpose
  - status
optional-fields:
  - owner
  - dependencies
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

	workflows := metadata.GetStringList("workflows")
	if workflows == nil || len(workflows) != 3 {
		t.Errorf("Expected 3 workflows, got %v", workflows)
		return
	}
	if workflows[0] != "generate" || workflows[1] != "validate" || workflows[2] != "execute" {
		t.Errorf("Workflows not parsed correctly: %v", workflows)
	}

	required := metadata.GetStringList("required-frontmatter")
	if required == nil || len(required) != 3 {
		t.Errorf("Expected 3 required fields, got %v", required)
	}

	optional := metadata.GetStringList("optional-fields")
	if optional == nil || len(optional) != 2 {
		t.Errorf("Expected 2 optional fields, got %v", optional)
	}
}

// TestRealWorldStatusValuesArray tests parsing of status values array.
func TestRealWorldStatusValuesArray(t *testing.T) {
	content := `---
name: test-skill
description: A skill with status enum
status-values:
  - pending
  - in_progress
  - completed
  - failed
  - on_hold
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

	statusValues := metadata.GetStringList("status-values")
	if statusValues == nil || len(statusValues) != 5 {
		t.Errorf("Expected 5 status values, got %v", statusValues)
		return
	}

	expectedStatuses := []string{"pending", "in_progress", "completed", "failed", "on_hold"}
	for i, expected := range expectedStatuses {
		if statusValues[i] != expected {
			t.Errorf("Expected status[%d]='%s', got '%s'", i, expected, statusValues[i])
		}
	}
}
