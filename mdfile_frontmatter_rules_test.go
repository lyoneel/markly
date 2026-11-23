package markly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frontmatterRuleCase is a document plus what every parser must agree on.
type frontmatterRuleCase struct {
	name     string
	doc      string
	wantMeta bool
	wantBody string // RawBody after LoadContent
}

var frontmatterRuleCases = []frontmatterRuleCase{
	{
		name:     "yaml block on line 1",
		doc:      "---\ntitle: T\n---\n# Body\n",
		wantMeta: true,
		wantBody: "# Body",
	},
	{
		name:     "toml block on line 1",
		doc:      "+++\ntitle = 'T'\n+++\n# Body\n",
		wantMeta: true,
		wantBody: "# Body",
	},
	{
		name:     "crlf block",
		doc:      "---\r\ntitle: T\r\n---\r\n# Body\r\n",
		wantMeta: true,
		wantBody: "# Body", // bufio.ScanLines drops the \r
	},
	{
		name:     "thematic breaks in a body without frontmatter",
		doc:      "# Title\n\nIntro.\n\n---\n\nrole: admin\n\n---\n\nOutro.\n",
		wantMeta: false,
		wantBody: "# Title\n\nIntro.\n\n---\n\nrole: admin\n\n---\n\nOutro.",
	},
	{
		name:     "leading blank line",
		doc:      "\n---\ntitle: T\n---\n# Body\n",
		wantMeta: false,
		wantBody: "\n---\ntitle: T\n---\n# Body",
	},
	{
		name:     "yaml opened, toml closed",
		doc:      "---\ntitle: T\n+++\n# Body\n",
		wantMeta: false,
		wantBody: "---\ntitle: T\n+++\n# Body",
	},
	{
		name:     "unclosed block",
		doc:      "---\ntitle: T\n# Body\n",
		wantMeta: false,
		wantBody: "---\ntitle: T\n# Body",
	},
	{
		name:     "body thematic break after real frontmatter",
		doc:      "---\ntitle: T\n---\n\nIntro.\n\n---\n\nrole: admin\n",
		wantMeta: true,
		wantBody: "\nIntro.\n\n---\n\nrole: admin",
	},
}

func TestFrontmatterRulesAgreeAcrossParsers(t *testing.T) {
	for _, tc := range frontmatterRuleCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "doc.md")
			writeFile(t, path, tc.doc)

			// File loader: metadata.
			md := NewMDFile(path)
			meta, err := md.GetMetadata()
			if err != nil {
				t.Fatalf("GetMetadata: %v", err)
			}
			if (meta != nil) != tc.wantMeta {
				t.Fatalf("GetMetadata found metadata = %v, want %v (%v)", meta != nil, tc.wantMeta, meta)
			}
			if meta != nil && meta.GetString("role") != "" {
				t.Fatalf("body text leaked into metadata: %v", meta.Data())
			}

			// File loader: content.
			if err := md.LoadContent(); err != nil {
				t.Fatalf("LoadContent: %v", err)
			}
			c, _ := md.GetContent()
			if c.RawBody != tc.wantBody {
				t.Fatalf("RawBody = %q, want %q", c.RawBody, tc.wantBody)
			}

			// In-memory loader.
			mem, _ := NewMDFileFromString(tc.doc).GetMetadata()
			if (mem != nil) != tc.wantMeta {
				t.Fatalf("NewMDFileFromString found metadata = %v, want %v", mem != nil, tc.wantMeta)
			}

			// ParseFrontmatter / SplitFrontmatter.
			_, _, found := SplitFrontmatter(tc.doc)
			if found != tc.wantMeta {
				t.Fatalf("SplitFrontmatter found = %v, want %v", found, tc.wantMeta)
			}
			pm, _, perr := ParseFrontmatter(tc.doc)
			if (pm != nil) != tc.wantMeta || (perr == nil) != tc.wantMeta {
				t.Fatalf("ParseFrontmatter = %v, %v; want metadata %v", pm, perr, tc.wantMeta)
			}
		})
	}
}

func TestSaveDoesNotDuplicateBodyThematicBreaks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	orig := "# Title\n\nIntro.\n\n---\n\nrole: admin\n\n---\n\nOutro.\n"
	writeFile(t, path, orig)

	md := NewMDFile(path)
	meta, err := md.GetMetadata()
	if err != nil || meta != nil {
		t.Fatalf("GetMetadata = %v, %v; want no metadata", meta, err)
	}
	// Plain round-trip keeps the file byte for byte.
	if err := md.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Fatalf("round-trip changed the file:\n%q\nwant\n%q", got, orig)
	}

	// Adding metadata writes one new block and keeps the body once.
	md = NewMDFile(path)
	md.SetMetadata(NewMDMetadataWithFormat(map[string]any{"title": "x"}, 0, 0, FMTypeYAML))
	if err := md.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if strings.Count(string(got), "role: admin") != 1 {
		t.Fatalf("body duplicated into metadata:\n%s", got)
	}
	re, err := NewMDFile(path).GetMetadata()
	if err != nil || re == nil || re.GetString("title") != "x" || re.Get("role") != nil {
		t.Fatalf("reloaded metadata = %v, %v", re, err)
	}
}

func TestSaveRoundTripWithFrontmatterAndBodyBreaks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	orig := "---\ntitle: T\n---\n\nIntro.\n\n---\n\nmore: text\n\n---\n"
	writeFile(t, path, orig)

	md := NewMDFile(path)
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatal(err)
	}
	meta.SetString("title", "U")
	if err := md.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "---\ntitle: U\n---\n\nIntro.\n\n---\n\nmore: text\n\n---\n"
	if string(got) != want {
		t.Fatalf("Save wrote\n%q\nwant\n%q", got, want)
	}
	re, _ := NewMDFile(path).GetMetadata()
	if re.Get("more") != nil || re.GetString("title") != "U" {
		t.Fatalf("reloaded metadata = %v", re.Data())
	}
}

func TestLeadingBlankLinesKeptOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	orig := "\n\n# Title\n\nbody\n"
	writeFile(t, path, orig)
	md := NewMDFile(path)
	if err := md.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Fatalf("Save wrote %q, want %q", got, orig)
	}
}
