package validate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const vocabFile = "```\nalpha\nbeta\n# comment\n```\n"

// vocabFixture builds tmp/project/schemas plus tmp/private/secrets.md and
// returns the schema directory and the temp root.
func vocabFixture(t *testing.T) (schemaDir, tmp string) {
	t.Helper()
	tmp = t.TempDir()
	schemaDir = filepath.Join(tmp, "project", "schemas")
	writeTestFile(t, filepath.Join(tmp, "private", "secrets.md"), "```\nhunter2\nAKIA-EXAMPLE\n```\n")
	writeTestFile(t, filepath.Join(schemaDir, "lists", "nested", "kinds.md"), vocabFile)
	writeTestFile(t, filepath.Join(schemaDir, "local.md"), vocabFile)
	return schemaDir, tmp
}

func schemaWithFrom(t *testing.T, dir, from string) string {
	t.Helper()
	p := filepath.Join(dir, "schema.md")
	writeTestFile(t, p, "---\nfields:\n  tag: {shape: single, from: '"+from+"'}\n---\n")
	return p
}

func assertVocabEscape(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrVocabularyEscape) || !errors.Is(err, ErrSchema) {
		t.Fatalf("LoadSchema error = %v; want ErrVocabularyEscape and ErrSchema", err)
	}
}

func TestVocabularyTraversalRefused(t *testing.T) {
	schemaDir, tmp := vocabFixture(t)
	for _, from := range []string{
		"../../private/secrets",
		"lists/../../../private/secrets",
		filepath.ToSlash(filepath.Join(tmp, "private", "secrets")),
		"../secrets",
	} {
		s, err := LoadSchema(schemaWithFrom(t, schemaDir, from))
		if s != nil {
			t.Fatalf("from %q: schema loaded with vocab %v", from, s.Vocab)
		}
		assertVocabEscape(t, err)
	}
}

func TestVocabularyInDirectoryRefsStillWork(t *testing.T) {
	schemaDir, _ := vocabFixture(t)
	for _, from := range []string{"local", "lists/nested/kinds", "./local"} {
		s, err := LoadSchema(schemaWithFrom(t, schemaDir, from))
		if err != nil {
			t.Fatalf("from %q: %v", from, err)
		}
		if v := s.Vocab[from]; !v["alpha"] || !v["beta"] || len(v) != 2 {
			t.Fatalf("from %q: vocab = %v", from, v)
		}
	}
}

func TestVocabularyBareRefFallbacksStillWork(t *testing.T) {
	schemaDir, tmp := vocabFixture(t)
	writeTestFile(t, filepath.Join(tmp, "project", "assets", "from-assets.md"), vocabFile)
	writeTestFile(t, filepath.Join(tmp, "project", "references", "from-refs.md"), vocabFile)
	writeTestFile(t, filepath.Join(tmp, "project", "from-parent.md"), vocabFile)
	for _, from := range []string{"from-assets", "from-refs", "from-parent"} {
		s, err := LoadSchema(schemaWithFrom(t, schemaDir, from))
		if err != nil {
			t.Fatalf("from %q: %v", from, err)
		}
		if !s.Vocab[from]["alpha"] {
			t.Fatalf("from %q: vocab = %v", from, s.Vocab[from])
		}
	}
}

func TestVocabularyEscapingSymlinkRefused(t *testing.T) {
	schemaDir, tmp := vocabFixture(t)
	if err := os.Symlink(filepath.Join(tmp, "private", "secrets.md"), filepath.Join(schemaDir, "leak.md")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	if err := os.Symlink(filepath.Join(tmp, "private"), filepath.Join(schemaDir, "privdir")); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{"leak", "privdir/secrets"} {
		_, err := LoadSchema(schemaWithFrom(t, schemaDir, from))
		assertVocabEscape(t, err)
	}
}

func TestVocabularyInRootSymlinkAllowed(t *testing.T) {
	schemaDir, _ := vocabFixture(t)
	if err := os.Symlink("local.md", filepath.Join(schemaDir, "alias.md")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	s, err := LoadSchema(schemaWithFrom(t, schemaDir, "alias"))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Vocab["alias"]["alpha"] {
		t.Fatalf("vocab = %v", s.Vocab["alias"])
	}
}

func TestVocabularySymlinkedSchemaDirectory(t *testing.T) {
	schemaDir, tmp := vocabFixture(t)
	link := filepath.Join(tmp, "linked-schemas")
	if err := os.Symlink(schemaDir, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	schemaWithFrom(t, schemaDir, "lists/nested/kinds")
	s, err := LoadSchema(filepath.Join(link, "schema.md"))
	if err != nil {
		t.Fatalf("LoadSchema through a symlinked schema dir: %v", err)
	}
	if !s.Vocab["lists/nested/kinds"]["beta"] {
		t.Fatalf("vocab = %v", s.Vocab)
	}
}

func TestVocabularyMissingStillNotFound(t *testing.T) {
	schemaDir, _ := vocabFixture(t)
	_, err := LoadSchema(schemaWithFrom(t, schemaDir, "lists/absent"))
	if !errors.Is(err, ErrSchema) || errors.Is(err, ErrVocabularyEscape) {
		t.Fatalf("LoadSchema error = %v; want a plain not-found schema error", err)
	}
}
