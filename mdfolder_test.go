package markly

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewMDFolder_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	if folder == nil {
		t.Fatal("Expected non-nil MDFolder")
	}

	files := folder.GetAll()
	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}
}

func TestNewMDFolder_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()

	testMD := `---
title: "Test File"
depends: ["dep1.md"]
---

# Heading 1

Content here.
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(testMD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	files := folder.GetAll()
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	var firstFile *MDFile
	for _, f := range files {
		firstFile = f
		break
	}
	if firstFile == nil {
		t.Fatal("firstFile is nil")
	}

	meta, err := firstFile.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta == nil {
		t.Fatal("Expected non-nil metadata")
	}

	if meta.GetString("title") != "Test File" {
		t.Errorf("Expected title 'Test File', got '%s'", meta.GetString("title"))
	}
}

func TestNewMDFolder_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
depends: ["c.md"]
---
# B`,
		"c.md": `---
title: "C"
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	allFiles := folder.GetAll()
	if len(allFiles) != 3 {
		t.Errorf("Expected 3 files, got %d", len(allFiles))
	}
}

func TestMDFolder_GetAll(t *testing.T) {
	tmpDir := t.TempDir()

	for i := 0; i < 5; i++ {
		content := fmt.Sprintf(`---
title: "File %d"
---
# File %d`, i, i)
		if err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.md", i)), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	files := folder.GetAll()
	if len(files) != 5 {
		t.Errorf("Expected 5 files, got %d", len(files))
	}
}

func TestMDFolder_Iterate(t *testing.T) {
	tmpDir := t.TempDir()

	for i := 0; i < 3; i++ {
		content := fmt.Sprintf(`---
title: "File%d"
---
# File%d`, i, i)
		if err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.md", i)), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	count := 0
	folder.Iterate(func(md *MDFile) {
		count++
	})

	if count != 3 {
		t.Errorf("Expected Iterate to call function 3 times, got %d", count)
	}
}

func TestMDFolder_GetByMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
type: "doc"
priority: 1
active: true
---
# A`,
		"b.md": `---
title: "B"
type: "skill"
priority: 2
active: false
---
# B`,
		"c.md": `---
title: "C"
type: "doc"
priority: 3
active: true
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	tests := []struct {
		name      string
		filters   map[string]any
		expectLen int
	}{
		{"filter by type=doc", map[string]any{"type": "doc"}, 2},
		{"filter by active=true", map[string]any{"active": true}, 2},
		{"filter by priority=1", map[string]any{"priority": 1}, 1},
		{"filter multiple", map[string]any{"type": "doc", "active": true}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := folder.GetByMetadata(tt.filters)
			if len(files) != tt.expectLen {
				t.Errorf("Expected %d files, got %d", tt.expectLen, len(files))
			}
		})
	}
}

func TestMDFolder_GetLoadOrder_SimpleChain(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
depends: ["c.md"]
---
# B`,
		"c.md": `---
title: "C"
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	order, err := folder.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder failed: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 files in order, got %d", len(order))
	}

	// C should come first (no deps), then B, then A
	expectedOrder := []string{"c", "b", "a"}
	for i, expected := range expectedOrder {
		if filepath.Base(order[i].path) != expected+".md" {
			t.Errorf("Expected %s at position %d, got %s", expected, i, filepath.Base(order[i].path))
		}
	}
}

func TestMDFolder_GetLoadOrder_DAG(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md", "c.md"]
---
# A`,
		"b.md": `---
title: "B"
---
# B`,
		"c.md": `---
title: "C"
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	order, err := folder.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder failed: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 files in order, got %d", len(order))
	}

	// B and C should come before A (order of B/C may vary)
	foundBOrC := false
	for i, md := range order {
		name := filepath.Base(md.path)
		t.Logf("%d: %s\n", i, name)
		if name == "a.md" {
			// Once we find a.md, no more files should appear after it
			for j := i + 1; j < len(order); j++ {
				t.Errorf("File %s should come before a.md", filepath.Base(order[j].path))
			}
			break
		} else if name == "b.md" || name == "c.md" {
			foundBOrC = true
		}
	}

	if !foundBOrC {
		t.Error("Expected to find b.md or c.md before a.md")
	}
}

func TestMDFolder_GetLoadOrder_CircularDependency(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
depends: ["c.md"]
---
# B`,
		"c.md": `---
title: "C"
depends: ["a.md"]
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	_, err = folder.GetLoadOrder()
	if err == nil {
		t.Error("Expected error for circular dependency")
	}
}

func TestMDFolder_DetectCycles_NoCycle(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
---
# B`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	cycles := folder.DetectCycles()
	if len(cycles) != 0 {
		t.Errorf("Expected no cycles, got %d: %v", len(cycles), cycles)
	}
}

func TestMDFolder_DetectCycles_WithCycle(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
depends: ["c.md"]
---
# B`,
		"c.md": `---
title: "C"
depends: ["a.md"]
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	cycles := folder.DetectCycles()
	if len(cycles) == 0 {
		t.Error("Expected to detect cycle")
	}

	// Check that cycle is properly formatted
	hasCycle := false
	for _, cycle := range cycles {
		if strings.Contains(cycle, "a") && strings.Contains(cycle, "b") && strings.Contains(cycle, "c") {
			hasCycle = true
			break
		}
	}

	if !hasCycle {
		t.Errorf("Expected cycle containing a,b,c, got: %v", cycles)
	}
}

func TestMDFolder_GetAllWithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
depends: ["c.md"]
---
# B`,
		"c.md": `---
title: "C"
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	deps, err := folder.GetAllWithDependencies("a.md", nil)
	if err != nil {
		t.Fatalf("GetAllWithDependencies failed: %v", err)
	}

	if len(deps) != 3 {
		t.Errorf("Expected 3 files (C, B, A), got %d", len(deps))
	}

	// C should come first, then B, then A
	expectedOrder := []string{"c.md", "b.md", "a.md"}
	for i, expected := range expectedOrder {
		if filepath.Base(deps[i].path) != expected {
			t.Errorf("Expected %s at position %d, got %s", expected, i, filepath.Base(deps[i].path))
		}
	}
}

func TestMDFolder_GetAllWithDependencies_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
title: "Standalone"
---
# Standalone`
	if err := os.WriteFile(filepath.Join(tmpDir, "standalone.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	deps, err := folder.GetAllWithDependencies("standalone.md", nil)
	if err != nil {
		t.Fatalf("GetAllWithDependencies failed: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("Expected 1 file, got %d", len(deps))
	}
}

func TestMDFolder_GetAllWithDependencies_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
title: "A"
depends: ["b.md"]
---
# A`
	if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	_, err = folder.GetAllWithDependencies("nonexistent.md", nil)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestMDFolder_GetAllWithDependencies_Circular(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
depends: ["a.md"]
---
# B`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	_, err = folder.GetAllWithDependencies("a.md", nil)
	if err == nil {
		t.Error("Expected error for circular dependency")
	}
}

func TestMDFolder_CustomDepKeys(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
prerequisites: ["b.md"]
---
# A`,
		"b.md": `---
title: "B"
---
# B`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir, WithDepKeys("prerequisites"))
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	order, err := folder.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder failed: %v", err)
	}

	if filepath.Base(order[0].path) != "b.md" {
		t.Errorf("Expected b.md first, got %s", filepath.Base(order[0].path))
	}
}

func TestMDFolder_CustomDepSeparator(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"a.md": `---
title: "A"
depends: "b.md; c.md"
---
# A`,
		"b.md": `---
title: "B"
---
# B`,
		"c.md": `---
title: "C"
---
# C`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir, WithDepSeparator("; "))
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	order, err := folder.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder failed: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 files in order, got %d", len(order))
	}
}

func TestMDFolder_Subdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	filesContent := map[string]string{
		"root.md":          `---\ntitle: "Root"\n---\n# Root`,
		"subdir/nested.md": `---\ntitle: "Nested"\ndepends: ["../root.md"]\n---\n# Nested`,
	}

	for name, content := range filesContent {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	files := folder.GetAll()
	if len(files) != 2 {
		t.Errorf("Expected 2 files (including nested), got %d", len(files))
	}
}

func TestMDFolder_SkipNonMDFiles(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"test.md":    `---\ntitle: "Test"\n---\n# Test`,
		"readme.txt": "This is not markdown",
		"data.json":  `{"key": "value"}`,
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	files := folder.GetAll()
	if len(files) != 1 {
		t.Errorf("Expected only 1 .md file, got %d (included non-md files?)", len(files))
	}
}

func TestMDFolder_ComplexDAG(t *testing.T) {
	tmpDir := t.TempDir()

	filesContent := map[string]string{
		"base.md":  "---\ntitle: \"Base\"\n---\n# Base",
		"core.md":  "---\ntitle: \"Core\"\ndepends: [\"base.md\"]\n---\n# Core",
		"utils.md": "---\ntitle: \"Utils\"\ndepends: [\"base.md\"]\n---\n# Utils",
		"app.md":   "---\ntitle: \"App\"\ndepends: [\"core.md\", \"utils.md\"]\n---\n# App",
		"cli.md":   "---\ntitle: \"CLI\"\ndepends: [\"app.md\"]\n---\n# CLI",
		"web.md":   "---\ntitle: \"Web\"\ndepends: [\"app.md\"]\n---\n# Web",
	}

	for name, content := range filesContent {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	folder, err := NewMDFolder(tmpDir)
	if err != nil {
		t.Fatalf("NewMDFolder failed: %v", err)
	}

	order, err := folder.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder failed: %v", err)
	}

	if len(order) != 6 {
		t.Errorf("Expected 6 files in order, got %d", len(order))
	}

	// Verify dependency constraints: base before core/utils, core+utils before app
	baseIdx := -1
	coreIdx := -1
	utilsIdx := -1
	appIdx := -1

	for i, md := range order {
		name := filepath.Base(md.path)
		switch name {
		case "base.md":
			baseIdx = i
		case "core.md":
			coreIdx = i
		case "utils.md":
			utilsIdx = i
		case "app.md":
			appIdx = i
		}
	}

	if baseIdx == -1 || coreIdx == -1 || utilsIdx == -1 || appIdx == -1 {
		t.Fatal("Missing expected files in order")
	}

	// Check: base must come before both core and utils
	if baseIdx >= coreIdx {
		t.Errorf("base.md (idx %d) should come before core.md (idx %d)", baseIdx, coreIdx)
	}
	if baseIdx >= utilsIdx {
		t.Errorf("base.md (idx %d) should come before utils.md (idx %d)", baseIdx, utilsIdx)
	}

	// Check: both core and utils must come before app
	if coreIdx >= appIdx {
		t.Errorf("core.md (idx %d) should come before app.md (idx %d)", coreIdx, appIdx)
	}
	if utilsIdx >= appIdx {
		t.Errorf("utils.md (idx %d) should come before app.md (idx %d)", utilsIdx, appIdx)
	}

	// Check: cli and web must come after app
	cliIdx := -1
	webIdx := -1
	for i, md := range order {
		name := filepath.Base(md.path)
		switch name {
		case "cli.md":
			cliIdx = i
		case "web.md":
			webIdx = i
		}
	}

	if cliIdx != -1 && appIdx >= cliIdx {
		t.Errorf("app.md (idx %d) should come before cli.md (idx %d)", appIdx, cliIdx)
	}
	if webIdx != -1 && appIdx >= webIdx {
		t.Errorf("app.md (idx %d) should come before web.md (idx %d)", appIdx, webIdx)
	}
}
