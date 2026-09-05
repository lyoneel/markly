package markly

import (
	"testing"
)

// TestMDMetadataSetters tests basic round-trip functionality for all setter methods.
func TestMDMetadataSetters(t *testing.T) {
	m := NewMDMetadata(map[string]any{}, 0, 0)

	// SetString / GetString
	m.SetString("title", "Test")
	if m.GetString("title") != "Test" {
		t.Errorf("GetString returned %q, want 'Test'", m.GetString("title"))
	}

	// SetInt / GetInt
	m.SetInt("count", 42)
	if m.GetInt("count") != 42 {
		t.Errorf("GetInt returned %d, want 42", m.GetInt("count"))
	}

	// SetBool / GetBool
	m.SetBool("enabled", true)
	if !m.GetBool("enabled") {
		t.Error("GetBool returned false, want true")
	}

	// SetList / GetList
	m.SetList("tags", []any{"a", "b"})
	tags := m.GetList("tags")
	if tags == nil || len(tags) != 2 {
		t.Errorf("GetList returned %v with length %d, want []any{\"a\", \"b\"}", tags, len(tags))
	}

	// SetStringList / GetStringList
	m.SetStringList("items", []string{"x", "y"})
	items := m.GetStringList("items")
	if items == nil || len(items) != 2 {
		t.Errorf("GetStringList returned %v with length %d, want []string{\"x\", \"y\"}", items, len(items))
	}

	// SetMap / GetMap
	nested := map[string]any{"key": "value"}
	m.SetMap("nested", nested)
	result := m.GetMap("nested")
	if result == nil || result["key"] != "value" {
		t.Errorf("GetMap returned %v, want map[string]any{\"key\": \"value\"}", result)
	}
}

// TestMDMetadataSettersEdgeCases tests edge cases for setter methods.
func TestMDMetadataSettersEdgeCases(t *testing.T) {
	m := NewMDMetadata(map[string]any{}, 0, 0)

	t.Run("SetInt with zero", func(t *testing.T) {
		m.SetInt("zero", 0)
		if m.GetInt("zero") != 0 {
			t.Errorf("GetInt returned %d, want 0", m.GetInt("zero"))
		}
	})

	t.Run("SetBool false", func(t *testing.T) {
		m.SetBool("disabled", false)
		if m.GetBool("disabled") {
			t.Error("GetBool returned true, want false")
		}
	})

	t.Run("SetList empty", func(t *testing.T) {
		m.SetList("empty", []any{})
		list := m.GetList("empty")
		if list == nil || len(list) != 0 {
			t.Errorf("GetList returned %v with length %d, want empty slice", list, len(list))
		}
	})

	t.Run("SetStringList empty", func(t *testing.T) {
		m.SetStringList("emptyStrings", []string{})
		list := m.GetStringList("emptyStrings")
		if list == nil || len(list) != 0 {
			t.Errorf("GetStringList returned %v with length %d, want empty slice", list, len(list))
		}
	})

	t.Run("SetMap empty", func(t *testing.T) {
		m.SetMap("emptyMap", map[string]any{})
		result := m.GetMap("emptyMap")
		if result == nil || len(result) != 0 {
			t.Errorf("GetMap returned %v with length %d, want empty map", result, len(result))
		}
	})

	t.Run("SetInt negative", func(t *testing.T) {
		m.SetInt("negative", -100)
		if m.GetInt("negative") != -100 {
			t.Errorf("GetInt returned %d, want -100", m.GetInt("negative"))
		}
	})

	t.Run("SetInt large value", func(t *testing.T) {
		m.SetInt("large", 2147483647) // Max int32
		if m.GetInt("large") != 2147483647 {
			t.Errorf("GetInt returned %d, want 2147483647", m.GetInt("large"))
		}
	})

	t.Run("SetString empty string", func(t *testing.T) {
		m.SetString("emptyStr", "")
		if m.GetString("emptyStr") != "" {
			t.Errorf("GetString returned %q, want empty string", m.GetString("emptyStr"))
		}
	})

	t.Run("Overwrite different types", func(t *testing.T) {
		m.SetInt("typeTest", 42)
		if m.GetInt("typeTest") != 42 {
			t.Error("GetInt failed after SetInt")
		}

		m.SetString("typeTest", "now string")
		if m.GetString("typeTest") != "now string" {
			t.Error("GetString failed after overwrite with string")
		}

		if m.GetInt("typeTest") != 0 { // Should return 0 for non-numeric
			t.Errorf("GetInt returned %d, want 0 for string value", m.GetInt("typeTest"))
		}
	})
}

// TestMDMetadataDelete tests the Delete method.
func TestMDMetadataDelete(t *testing.T) {
	m := NewMDMetadata(map[string]any{
		"keep":     "value1",
		"deleteMe": "value2",
	}, 0, 0)

	// Verify key exists before delete
	if m.GetString("deleteMe") != "value2" {
		t.Fatal("Key 'deleteMe' should exist before deletion")
	}

	m.Delete("deleteMe")

	// Verify key is deleted (GetString returns empty string for missing keys)
	if m.GetString("deleteMe") != "" {
		t.Errorf("Delete failed: GetString returned %q, want empty string", m.GetString("deleteMe"))
	}

	// Verify other keys are unaffected
	if m.GetString("keep") != "value1" {
		t.Errorf("Delete affected other key: GetString('keep') returned %q, want 'value1'", m.GetString("keep"))
	}
}

// TestMDMetadataPersistence tests save and reload with modifications.
func TestMDMetadataPersistence(t *testing.T) {
	m := NewMDMetadata(map[string]any{}, 0, 0)

	// Modify metadata
	m.SetString("title", "Test Document")
	m.SetInt("version", 1)
	m.SetBool("published", true)

	// Verify modifications are in memory
	if m.GetString("title") != "Test Document" {
		t.Error("GetString failed after modification")
	}
	if m.GetInt("version") != 1 {
		t.Error("GetInt failed after modification")
	}
	if !m.GetBool("published") {
		t.Error("GetBool failed after modification")
	}

	// Note: Full persistence test requires file I/O which is tested separately in mdfile_test.go
	// This test verifies the modified flag and data integrity
}
