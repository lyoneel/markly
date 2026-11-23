package markly

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewMDFolderDefaultSymlinkPolicy(t *testing.T) {
	folder, err := NewMDFolder(t.TempDir())
	if err != nil {
		t.Fatalf("NewMDFolder: %v", err)
	}
	if got := folder.SymlinkPolicy(); got != FollowWithinRoot {
		t.Fatalf("default policy = %v, want FollowWithinRoot", got)
	}
	if DefaultSymlinkPolicy != FollowWithinRoot {
		t.Fatalf("DefaultSymlinkPolicy = %v, want FollowWithinRoot", DefaultSymlinkPolicy)
	}
}

func TestWithSymlinkPolicySetsPolicy(t *testing.T) {
	for _, policy := range []SymlinkPolicy{FollowWithinRoot, FollowAny, NoFollow} {
		folder, err := NewMDFolder(t.TempDir(), WithSymlinkPolicy(policy))
		if err != nil {
			t.Fatalf("NewMDFolder(%v): %v", policy, err)
		}
		if got := folder.SymlinkPolicy(); got != policy {
			t.Fatalf("policy = %v, want %v", got, policy)
		}
	}
}

func TestWithSymlinkPolicyRejectsUnknownValue(t *testing.T) {
	folder, err := NewMDFolder(t.TempDir(), WithSymlinkPolicy(SymlinkPolicy(42)))
	if !errors.Is(err, ErrInvalidSymlinkPolicy) {
		t.Fatalf("err = %v, want ErrInvalidSymlinkPolicy", err)
	}
	if folder != nil {
		t.Fatalf("folder = %v, want nil on error", folder)
	}
}

func TestMDFolderReadsPlainFilesUnderEveryPolicy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntitle: a\n---\n# A\n")
	writeFile(t, filepath.Join(dir, "sub", "b.MD"), "---\ntitle: b\n---\n# B\n")

	for _, policy := range []SymlinkPolicy{FollowWithinRoot, FollowAny, NoFollow} {
		folder, err := NewMDFolder(dir, WithSymlinkPolicy(policy))
		if err != nil {
			t.Fatalf("NewMDFolder: %v", err)
		}
		files := folder.GetAll()
		if len(files) != 2 || len(folder.Errors()) != 0 {
			t.Fatalf("%v: files=%d errors=%v, want 2 files and no errors", policy, len(files), folder.Errors())
		}
		md := files[filepath.Join("sub", "b.MD")]
		if md == nil {
			t.Fatalf("%v: sub/b.MD missing from %v", policy, files)
		}
		if err := md.LoadContent(); err != nil {
			t.Fatalf("%v: LoadContent: %v", policy, err)
		}
		content, _ := md.GetContent()
		if len(content.Headings) != 1 || content.Headings[0].Text != "B" {
			t.Fatalf("%v: headings = %+v", policy, content.Headings)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
