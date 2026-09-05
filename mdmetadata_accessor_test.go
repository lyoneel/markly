package markly

import (
	"testing"
)

// TestMetadataAccessors tests all the getter methods on MDMetadata.
func TestMetadataAccessors(t *testing.T) {
	metadata := NewMDMetadataSimple(map[string]any{
		"name":        "test-doc",
		"description": "A test document",
		"count":       42,
		"enabled":     true,
		"tags":        []any{"feature", "critical"},
		"nested": map[string]any{
			"key1": "value1",
			"key2": 100,
		},
	})

	tests := []struct {
		name  string
		check func(*MDMetadata) bool
	}{
		{
			name: "GetString on existing key",
			check: func(m *MDMetadata) bool {
				return m.GetString("name") == "test-doc"
			},
		},
		{
			name: "GetString on missing key",
			check: func(m *MDMetadata) bool {
				return m.GetString("nonexistent") == ""
			},
		},
		{
			name: "GetInt on existing numeric key",
			check: func(m *MDMetadata) bool {
				return m.GetInt("count") == 42
			},
		},
		{
			name: "GetInt on non-numeric value",
			check: func(m *MDMetadata) bool {
				return m.GetInt("name") == 0 // Should return 0 for string
			},
		},
		{
			name: "GetBool on existing boolean key",
			check: func(m *MDMetadata) bool {
				return m.GetBool("enabled") == true
			},
		},
		{
			name: "GetBool on non-bool value",
			check: func(m *MDMetadata) bool {
				return m.GetBool("name") == false // Should return false for string
			},
		},
		{
			name: "GetList returns correct type",
			check: func(m *MDMetadata) bool {
				list := m.GetList("tags")
				if list == nil || len(list) != 2 {
					return false
				}
				return true
			},
		},
		{
			name: "GetList returns wrong type",
			check: func(m *MDMetadata) bool {
				list := m.GetList("name") // name is a string, not a list
				return list == nil
			},
		},
		{
			name: "GetStringList extracts strings correctly",
			check: func(m *MDMetadata) bool {
				tags := m.GetStringList("tags")
				if tags == nil || len(tags) != 2 {
					return false
				}
				return tags[0] == "feature" && tags[1] == "critical"
			},
		},
		{
			name: "GetMap returns nested map",
			check: func(m *MDMetadata) bool {
				nested := m.GetMap("nested")
				if nested == nil {
					return false
				}
				return nested["key1"] == "value1" && nested["key2"] == 100
			},
		},
		{
			name: "GetMap returns wrong type",
			check: func(m *MDMetadata) bool {
				nested := m.GetMap("name") // name is a string, not a map
				return nested == nil
			},
		},
		{
			name: "Keys returns all top-level keys",
			check: func(m *MDMetadata) bool {
				keys := m.Keys()
				if len(keys) != 6 { // name, description, count, enabled, tags, nested = 6
					return false
				}
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(metadata) {
				t.Errorf("Metadata accessor check failed for test: %s", tt.name)
			}
		})
	}
}

// TestUnmarshalInto tests the UnmarshalInto method with custom structs.
func TestUnmarshalInto(t *testing.T) {
	type CustomStruct struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Count       int    `yaml:"count"`
	}

	metadata := NewMDMetadataSimple(map[string]any{
		"name":        "test-doc",
		"description": "A test document",
		"count":       42,
	})

	var result CustomStruct
	err := metadata.UnmarshalInto(&result)

	if err != nil {
		t.Fatalf("UnmarshalInto failed: %v", err)
	}

	if result.Name != "test-doc" || result.Description != "A test document" || result.Count != 42 {
		t.Errorf("UnmarshalInto did not populate struct correctly: %+v", result)
	}
}

// TestGetRawValue tests the Get method for raw value access.
func TestGetRawValue(t *testing.T) {
	metadata := NewMDMetadataSimple(map[string]any{
		"name":  "test-doc",
		"count": 42,
	})

	nameVal := metadata.Get("name")
	if nameVal != "test-doc" {
		t.Errorf("Get returned wrong value for 'name': %v", nameVal)
	}

	countVal := metadata.Get("count")
	if countVal != 42 {
		t.Errorf("Get returned wrong value for 'count': %v", countVal)
	}

	missingVal := metadata.Get("nonexistent")
	if missingVal != nil {
		t.Errorf("Get should return nil for missing key, got: %v", missingVal)
	}
}
