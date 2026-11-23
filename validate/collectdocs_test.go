package validate

import (
	"path/filepath"
	"reflect"
	"testing"
)

func collectFixture(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, p := range []string{"docs/a.md", "docs/b.md", "docs/sub/c.md", "top.md", "docs/skip.txt"} {
		writeTestFile(t, filepath.Join(dir, filepath.FromSlash(p)), "---\nx: 1\n---\n")
	}
	t.Chdir(dir)
}

func slashAll(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.ToSlash(p)
	}
	return out
}

func TestCollectDocsDotSlashGlobs(t *testing.T) {
	collectFixture(t)
	cases := map[string][]string{
		"docs/*.md":       {"docs/a.md", "docs/b.md"},
		"./docs/*.md":     {"docs/a.md", "docs/b.md"},
		"./docs/./*.md":   {"docs/a.md", "docs/b.md"},
		".//docs//*.md":   {"docs/a.md", "docs/b.md"},
		"./docs/**/*.md":  {"docs/a.md", "docs/b.md", "docs/sub/c.md"},
		"docs/**/*.md":    {"docs/a.md", "docs/b.md", "docs/sub/c.md"},
		"./**/c.md":       {"docs/sub/c.md"},
		"./*.md":          {"top.md"},
		"./docs/sub/?.md": {"docs/sub/c.md"},
	}
	for pattern, want := range cases {
		got, err := CollectDocs(pattern)
		if err != nil {
			t.Fatalf("CollectDocs(%q): %v", pattern, err)
		}
		if !reflect.DeepEqual(slashAll(got), want) {
			t.Fatalf("CollectDocs(%q) = %v, want %v", pattern, got, want)
		}
	}
}

func TestCollectDocsNoMatchStillErrors(t *testing.T) {
	collectFixture(t)
	for _, pattern := range []string{"./nope/*.md", "./docs/*.yaml"} {
		if got, err := CollectDocs(pattern); err == nil {
			t.Fatalf("CollectDocs(%q) = %v; want no documents matched", pattern, got)
		}
	}
}

func TestCollectDocsDotSlashDirectoryAndFile(t *testing.T) {
	collectFixture(t)
	got, err := CollectDocs("./docs")
	if err != nil || len(got) != 3 {
		t.Fatalf("CollectDocs(./docs) = %v, %v", got, err)
	}
	got, err = CollectDocs("./top.md")
	if err != nil || len(got) != 1 {
		t.Fatalf("CollectDocs(./top.md) = %v, %v", got, err)
	}
}
