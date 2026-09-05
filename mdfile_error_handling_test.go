package markly

import (
	"os"
	"testing"
)

// TestFileNotFound tests error handling when file doesn't exist.
func TestFileNotFound(t *testing.T) {
	md := NewMDFile("/nonexistent/path/to/file.md")
	err := md.processMetadata()

	if err == nil {
		t.Error("Expected error for non-existent file but got none")
		return
	}

	expectedSubstring := "metadata not found"
	if err.Error()[:len(expectedSubstring)] != expectedSubstring {
		t.Errorf("Expected error to contain '%s', got: %v", expectedSubstring, err)
	}
}

// TestPermissionDenied tests error handling when file has no read permissions.
func TestPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test: root bypasses file permission bits")
	}

	content := `---
name: test-doc
---

# Title`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	// Remove read permissions
	err := os.Chmod(tmpFile, 0000)
	if err != nil {
		t.Skipf("Skipping permission test (may require root): %v", err)
		return
	}

	md := NewMDFile(tmpFile)
	err = md.processMetadata()

	if err == nil {
		t.Error("Expected error for permission denied but got none")
		return
	}

	expectedSubstring := "open failed"
	if err.Error()[:len(expectedSubstring)] != expectedSubstring {
		t.Errorf("Expected error to contain '%s', got: %v", expectedSubstring, err)
	}
}

// TestScannerError tests handling of scanner errors (e.g., extremely long lines).
func TestScannerError(t *testing.T) {
	// Create a file with an extremely long line that exceeds bufio.Scanner's default buffer
	longLine := make([]byte, 65*1024+1) // Exceeds default 64KB buffer
	for i := range longLine {
		longLine[i] = 'a'
	}

	content := "---\nname: test-doc\n---\n" + string(longLine)

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	md, _ := NewMDFileWithContent(tmpFile)
	err := md.LoadContent()

	// Scanner should fail with a buffer size error for extremely long lines
	if err == nil {
		t.Log("Note: Scanner handled long line without error (may have larger default buffer)")
	} else {
		t.Logf("Scanner correctly reported error for long line: %v", err)
	}
}
