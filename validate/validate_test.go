package validate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/lyoneel/markly"
)

func TestLoadSchemaThreadsFixture(t *testing.T) {
	schema, err := LoadSchema("testdata/schema-threads.md")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	wantOrder := []string{"date", "updated", "type", "status", "keywords", "description", "sources", "conclusions", "related", "tools", "hardware", "games", "hosts", "notes", "region", "domain", "session"}
	got := make([]string, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		got = append(got, f.Name)
	}
	if !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("field order = %v, want %v", got, wantOrder)
	}
	if len(schema.Config) != 1 {
		t.Errorf("config = %v, want one sections entry", schema.Config)
	}
	sections, ok := schema.Config["sections"].([]any)
	if !ok || len(sections) != 3 {
		t.Errorf("sections config = %v, want the three headings", schema.Config["sections"])
	}
	session := schema.Fields[16]
	if session.Shape != "object" || len(session.Keys) != 4 {
		t.Errorf("session field = %+v, want object shape with 4 keys", session)
	}
}

func TestLoadSchemaGamesFixture(t *testing.T) {
	path := "testdata/games-catalog/references/schema-games.md"
	schema, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if len(schema.Fields) != 13 {
		t.Errorf("fields = %d, want 13", len(schema.Fields))
	}
	if schema.Config["tag-list-heading"] != "## Tag List" || schema.Config["overview-heading"] != "## Overview" {
		t.Errorf("config = %v, want both heading constants", schema.Config)
	}
	for _, ref := range []string{"controlled-stores", "controlled-features", "controlled-dei-level", "controlled-controller-support", "controlled-other-features", "controlled-dlcs-kind"} {
		if len(schema.Vocab[ref]) == 0 {
			t.Errorf("vocabulary %s = empty, want resolved entries", ref)
		}
	}
	// tag-vocabulary stores its list after a heading, not inside code
	// fences, so the engine resolves it empty; the games gate overrides
	// the entry with its own heading-based loader.
	tags, resolved := schema.Vocab["tag-vocabulary"]
	if !resolved || len(tags) != 0 {
		t.Errorf("tag-vocabulary = %v resolved=%v, want an empty resolved set", tags, resolved)
	}
	schema.Vocab["tag-vocabulary"] = map[string]bool{"Indie": true, "Action": true}
	if !schema.Vocab["tag-vocabulary"]["Indie"] {
		t.Error("vocab override did not stick")
	}
	if !schema.Vocab["controlled-stores"]["value-one"] {
		t.Errorf("controlled-stores vocab = %v, want the fixture values", schema.Vocab["controlled-stores"])
	}
}

func TestLoadSchemaDemoFromAssets(t *testing.T) {
	schema, err := LoadSchema("testdata/demo/references/schema-demo.md")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if !schema.Vocab["constant-kind"]["a"] {
		t.Errorf("constant-kind vocab = %v, want values from the assets folder", schema.Vocab["constant-kind"])
	}
}

func TestLoadSchemaYAMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yaml")
	content := "fields:\n  title: {required: true, shape: string}\n  kind: {required: true, shape: single, values: [a, b]}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	schema, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if len(schema.Fields) != 2 || schema.Fields[0].Name != "title" {
		t.Errorf("fields = %v, want title then kind", schema.Fields)
	}
}

func TestLoadSchemaErrors(t *testing.T) {
	dir := t.TempDir()

	noFields := filepath.Join(dir, "no-fields.md")
	if err := os.WriteFile(noFields, []byte("---\nconfig: {}\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSchema(noFields); !errors.Is(err, ErrSchema) {
		t.Errorf("no fields: err = %v, want ErrSchema", err)
	}

	missingVocab := filepath.Join(dir, "missing-vocab.md")
	if err := os.WriteFile(missingVocab, []byte("---\nfields:\n  k: {shape: single, from: absent-list}\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSchema(missingVocab)
	if !errors.Is(err, ErrSchema) || !stringsContains(err.Error(), "absent-list") {
		t.Errorf("missing vocab: err = %v, want ErrSchema naming absent-list", err)
	}

	brokenYaml := filepath.Join(dir, "broken.md")
	if err := os.WriteFile(brokenYaml, []byte("---\nfields: [unclosed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSchema(brokenYaml); !errors.Is(err, ErrSchema) {
		t.Errorf("broken yaml: err = %v, want ErrSchema", err)
	}
}

func TestValidateDocumentShapes(t *testing.T) {
	schema, err := LoadSchema("testdata/schema-threads.md")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}

	good := map[string]any{
		"date":        20260922,
		"updated":     "20260922",
		"type":        "thread",
		"status":      "active",
		"keywords":    []any{"go", "markdown"},
		"description": "One sentence.",
		"sources":     []any{map[string]any{"name": "n", "url": "u", "title": "t"}},
		"conclusions": []any{"claim"},
	}
	if issues := schema.ValidateDocument("x.md", good); len(issues) != 0 {
		t.Errorf("good document produced issues: %v", issues)
	}

	tests := []struct {
		name      string
		field     string
		value     any
		wantField string
		wantPart  string
	}{
		{name: "int date passes", field: "date", value: 20260922, wantField: "", wantPart: ""},
		{name: "bad month date", field: "updated", value: "202613011", wantField: "updated", wantPart: "must be a YYYYMMDD date string"},
		{name: "date bad month", field: "updated", value: "20261301", wantField: "updated", wantPart: "must be a YYYYMMDD date string"},
		{name: "date bad day", field: "updated", value: "20260132", wantField: "updated", wantPart: "must be a YYYYMMDD date string"},
		{name: "date bool", field: "updated", value: true, wantField: "updated", wantPart: "must be a YYYYMMDD date string"},
		{name: "single membership", field: "type", value: "note", wantField: "type", wantPart: "invalid type value 'note'"},
		{name: "single type error", field: "type", value: 5, wantField: "type", wantPart: "must be a string"},
		{name: "list entry empty", field: "keywords", value: []any{"a", ""}, wantField: "keywords", wantPart: "entries must be non-empty strings"},
		{name: "list type", field: "keywords", value: "a", wantField: "keywords", wantPart: "must be a list"},
		{name: "list empty ok", field: "related", value: []any{}, wantField: "", wantPart: ""},
		{name: "string type", field: "description", value: 5, wantField: "description", wantPart: "must be a string"},
		{name: "string empty ok", field: "description", value: "", wantField: "", wantPart: ""},
		{name: "objects missing key", field: "sources", value: []any{map[string]any{"name": "n"}}, wantField: "sources", wantPart: "missing required key 'url'"},
		{name: "objects empty string key counts as present", field: "sources", value: []any{map[string]any{"name": "", "url": "u", "title": "t"}}, wantField: "", wantPart: ""},
		{name: "objects bare string", field: "sources", value: []any{"bare"}, wantField: "sources", wantPart: "must be a non-empty map"},
		{name: "objects empty list ok", field: "sources", value: []any{}, wantField: "", wantPart: ""},
		{name: "object missing key", field: "session", value: map[string]any{"id": "s"}, wantField: "session", wantPart: "missing required key 'uuid'"},
		{name: "object empty string key counts as present", field: "session", value: map[string]any{"id": "", "uuid": "", "title": "", "data_dir": ""}, wantField: "", wantPart: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{}
			for k, v := range good {
				data[k] = v
			}
			data[tt.field] = tt.value
			issues := schema.ValidateDocument("x.md", data)
			if tt.wantPart == "" {
				if len(issues) != 0 {
					t.Errorf("value %v produced issues: %v", tt.value, issues)
				}
				return
			}
			found := false
			for _, issue := range issues {
				if issue.Field == tt.wantField && contains(issue.Reason, tt.wantPart) {
					found = true
				}
			}
			if !found {
				t.Errorf("value %v produced issues %v, want field %q reason containing %q", tt.value, issues, tt.wantField, tt.wantPart)
			}
		})
	}
}

func TestValidateDocumentMissingRequired(t *testing.T) {
	schema, err := LoadSchema("testdata/schema-threads.md")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	issues := schema.ValidateDocument("x.md", map[string]any{})
	if len(issues) != 8 {
		t.Fatalf("issues = %d, want 8 (the required fields)", len(issues))
	}
	for _, issue := range issues {
		if issue.File != "x.md" {
			t.Errorf("issue file = %q, want x.md", issue.File)
		}
	}
	if issues[0].Field != "date" || !contains(issues[0].Reason, "missing required field date") {
		t.Errorf("first issue = %+v, want missing date in schema order", issues[0])
	}
}

func TestValidateDocumentUnsupportedShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.md")
	if err := os.WriteFile(path, []byte("---\nfields:\n  t: {shape: bogus}\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	schema, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	issues := schema.ValidateDocument("x.md", map[string]any{"t": "v"})
	if len(issues) != 1 || !contains(issues[0].Reason, "unsupported shape") {
		t.Errorf("issues = %v, want one unsupported shape issue", issues)
	}
}

func TestRegisterShapeOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.md")
	if err := os.WriteFile(path, []byte("---\nfields:\n  t: {required: true, shape: loud}\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	schema, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	var gotCtx any = ""
	schema.Ctx = "ctx-value"
	schema.RegisterShape("loud", func(ctx any, field Field, value any) []Issue {
		gotCtx = ctx
		return []Issue{{Field: field.Name, Reason: "loud issue"}}
	})
	issues := schema.ValidateDocument("x.md", map[string]any{"t": "v"})
	if len(issues) != 1 || issues[0].Reason != "loud issue" || issues[0].File != "x.md" {
		t.Errorf("issues = %v, want the registered shape issue stamped with the file", issues)
	}
	if gotCtx != "ctx-value" {
		t.Errorf("ctx = %v, want ctx-value", gotCtx)
	}
}

func TestValidateDirFileGlobAndDirectory(t *testing.T) {
	schema, err := LoadSchema("testdata/demo/references/schema-demo.md")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	ok := "---\ntitle: Thing\nkind: a\n---\nbody\n"
	bad := "---\ntitle: Thing\nkind: z\n---\nbody\n"
	for name, content := range map[string]string{"ok.md": ok, "bad.md": bad} {
		if err := os.WriteFile(filepath.Join(docs, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("directory", func(t *testing.T) {
		issues, err := schema.ValidateDir(docs)
		if err != nil {
			t.Fatalf("ValidateDir: %v", err)
		}
		if len(issues) != 1 || issues[0].File != "bad.md" || !contains(issues[0].Reason, "invalid kind value 'z'") {
			t.Errorf("issues = %v, want one kind membership issue", issues)
		}
	})

	t.Run("single file", func(t *testing.T) {
		issues, err := schema.ValidateDir(filepath.Join(docs, "ok.md"))
		if err != nil {
			t.Fatalf("ValidateDir: %v", err)
		}
		if len(issues) != 0 {
			t.Errorf("issues = %v, want none", issues)
		}
	})

	t.Run("glob", func(t *testing.T) {
		issues, err := schema.ValidateDir(filepath.Join(root, "docs", "*.md"))
		if err != nil {
			t.Fatalf("ValidateDir: %v", err)
		}
		if len(issues) != 1 {
			t.Errorf("issues = %v, want one", issues)
		}
	})

	t.Run("no match is an error", func(t *testing.T) {
		if _, err := schema.ValidateDir(filepath.Join(root, "none", "*.md")); err == nil {
			t.Error("ValidateDir = nil error, want no documents matched")
		}
	})

	t.Run("document without frontmatter reports an issue", func(t *testing.T) {
		bare := filepath.Join(docs, "bare.md")
		if err := os.WriteFile(bare, []byte("no frontmatter here\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(bare)
		issues, err := schema.ValidateDir(bare)
		if err != nil {
			t.Fatalf("ValidateDir: %v", err)
		}
		if len(issues) != 1 || issues[0].Reason != "no YAML frontmatter found" || issues[0].Field != "" {
			t.Errorf("issues = %v, want one no-frontmatter issue", issues)
		}
	})
}

func contains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 || indexOf(s, part) >= 0)
}

func indexOf(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}

func stringsContains(s, part string) bool {
	return indexOf(s, part) >= 0
}

func inlineTestSchema(t *testing.T) *Schema {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.md")
	content := "---\nfields:\n  tags: {required: true, shape: list, inline: true}\n  sources: {required: false, shape: objects, inline: true}\n  meta: {required: false, shape: object, inline: true}\n  price: {required: false, shape: case, inline: true}\n  title: {required: false, shape: string, inline: true}\n  plain: {required: false, shape: list}\n---\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	schema, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	return schema
}

func metadataFromDoc(t *testing.T, text string) *markly.MDMetadata {
	t.Helper()
	meta, _, err := markly.ParseFrontmatter(text)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	return meta
}

func TestValidateMetadataInline(t *testing.T) {
	schema := inlineTestSchema(t)

	tests := []struct {
		name      string
		text      string
		wantField string
		wantPart  string
	}{
		{
			name: "flow list passes",
			text: "---\ntags: [a, b]\n---\n",
		},
		{
			name:      "block list flags",
			text:      "---\ntags:\n  - a\n  - b\n---\n",
			wantField: "tags",
			wantPart:  "must be an inline list",
		},
		{
			name: "flow objects pass",
			text: "---\ntags: [a]\nsources: [{name: n, url: u}]\n---\n",
		},
		{
			name:      "block objects flag",
			text:      "---\ntags: [a]\nsources:\n  - name: n\n    url: u\n---\n",
			wantField: "sources",
			wantPart:  "must be an inline list",
		},
		{
			name: "flow object map passes",
			text: "---\ntags: [a]\nmeta: {id: s1}\n---\n",
		},
		{
			name:      "block object map flags",
			text:      "---\ntags: [a]\nmeta:\n  id: s1\n---\n",
			wantField: "meta",
			wantPart:  "must be an inline map",
		},
		{
			name: "flow case map passes",
			text: "---\ntags: [a]\nprice: {usd: \"9.99\"}\n---\n",
		},
		{
			name:      "block case map flags",
			text:      "---\ntags: [a]\nprice:\n  usd: 9.99\n---\n",
			wantField: "price",
			wantPart:  "must be an inline map",
		},
		{
			name: "plain string passes the inline string rule",
			text: "---\ntags: [a]\ntitle: one line\n---\n",
		},
		{
			name: "quoted string passes the inline string rule",
			text: "---\ntags: [a]\ntitle: 'also one line'\n---\n",
		},
		{
			name:      "block scalar string flags",
			text:      "---\ntags: [a]\ntitle: |\n  line one\n  line two\n---\n",
			wantField: "title",
			wantPart:  "must be an inline string",
		},
		{
			name: "missing optional inline field stays silent",
			text: "---\ntags: [a]\n---\n",
		},
		{
			name:      "missing required inline list reports only the shape issue",
			text:      "---\n---\n",
			wantField: "tags",
			wantPart:  "missing required field tags",
		},
		{
			name:      "non-list value under inline reports only the shape issue",
			text:      "---\ntags: not-a-list\n---\n",
			wantField: "tags",
			wantPart:  "must be a list",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := metadataFromDoc(t, tt.text)
			issues := schema.ValidateMetadata("x.md", meta)
			if tt.wantPart == "" {
				if len(issues) != 0 {
					t.Errorf("issues = %v, want none", issues)
				}
				return
			}
			found := false
			for _, issue := range issues {
				if issue.Field == tt.wantField && strings.Contains(issue.Reason, tt.wantPart) {
					found = true
				}
			}
			if !found {
				t.Errorf("issues = %v, want field %q reason containing %q", issues, tt.wantField, tt.wantPart)
			}
		})
	}
}

// TestValidateDocumentIgnoresInline documents the boundary: the plain
// data map carries no style information, so the inline rule applies
// only through ValidateMetadata.
func TestValidateDocumentIgnoresInline(t *testing.T) {
	schema := inlineTestSchema(t)
	issues := schema.ValidateDocument("x.md", map[string]any{"tags": []any{"a", "b"}})
	if len(issues) != 0 {
		t.Errorf("issues = %v, want none (style needs ValidateMetadata)", issues)
	}
}

// TestValidateMetadataInlineTOMLSkip verifies the style rule skips
// TOML frontmatter, which has no block style.
func TestValidateMetadataInlineTOMLSkip(t *testing.T) {
	schema := inlineTestSchema(t)
	meta := metadataFromDoc(t, "+++\ntags = ['a', 'b']\n+++\n")
	if issues := schema.ValidateMetadata("x.md", meta); len(issues) != 0 {
		t.Errorf("issues = %v, want none (TOML has no block style)", issues)
	}
}

// TestValidateMetadataUndeclaredInlineStaysSilent verifies fields
// without the inline rule accept both styles.
func TestValidateMetadataUndeclaredInlineStaysSilent(t *testing.T) {
	schema := inlineTestSchema(t)
	meta := metadataFromDoc(t, "---\ntags: [a]\nplain:\n  - x\n  - y\n---\n")
	if issues := schema.ValidateMetadata("x.md", meta); len(issues) != 0 {
		t.Errorf("issues = %v, want none (inline not declared on plain)", issues)
	}
}
