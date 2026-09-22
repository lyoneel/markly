package markly

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantRaw   string
		wantBody  string
		wantFound bool
	}{
		{
			name:      "no frontmatter",
			text:      "# Heading\n\nbody text\n",
			wantRaw:   "",
			wantBody:  "# Heading\n\nbody text\n",
			wantFound: false,
		},
		{
			name:      "empty text",
			text:      "",
			wantRaw:   "",
			wantBody:  "",
			wantFound: false,
		},
		{
			name:      "frontmatter with body",
			text:      "---\ntitle: T\ncount: 3\n---\n\nbody\n",
			wantRaw:   "title: T\ncount: 3",
			wantBody:  "\nbody\n",
			wantFound: true,
		},
		{
			name:      "frontmatter without body",
			text:      "---\ntitle: T\n---\n",
			wantRaw:   "title: T",
			wantBody:  "",
			wantFound: true,
		},
		{
			name:      "frontmatter with empty body section",
			text:      "---\ntitle: T\n---\n\n\n",
			wantRaw:   "title: T",
			wantBody:  "\n\n",
			wantFound: true,
		},
		{
			name:      "empty frontmatter",
			text:      "---\n---\nbody\n",
			wantRaw:   "",
			wantBody:  "body\n",
			wantFound: true,
		},
		{
			name:      "crlf input",
			text:      "---\r\ntitle: T\r\n---\r\nbody\r\n",
			wantRaw:   "title: T",
			wantBody:  "body\n",
			wantFound: true,
		},
		{
			name:      "toml format",
			text:      "+++\ntitle = 'T'\n+++\nbody\n",
			wantRaw:   "title = 'T'",
			wantBody:  "body\n",
			wantFound: true,
		},
		{
			name:      "unclosed frontmatter",
			text:      "---\ntitle: T\nmore text\n",
			wantRaw:   "title: T\nmore text\n",
			wantBody:  "",
			wantFound: true,
		},
		{
			name:      "delimiter only on a later line is body text",
			text:      "# Heading\n\n---\n\nnot frontmatter\n",
			wantRaw:   "",
			wantBody:  "# Heading\n\n---\n\nnot frontmatter\n",
			wantFound: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, body, found := SplitFrontmatter(tt.text)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if raw != tt.wantRaw {
				t.Errorf("raw = %q, want %q", raw, tt.wantRaw)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestSplitFrontmatterRoundTrip(t *testing.T) {
	text := "---\ntitle: T\n---\n\nbody\n"
	raw, body, found := SplitFrontmatter(text)
	if !found {
		t.Fatal("found = false, want true")
	}
	rebuilt := "---\n" + raw + "\n---\n" + body
	if rebuilt != text {
		t.Errorf("round trip = %q, want %q", rebuilt, text)
	}
	if reflect.DeepEqual(raw, body) {
		t.Errorf("raw and body must differ for %q", text)
	}
}

func TestParseFrontmatter(t *testing.T) {
	t.Run("yaml document", func(t *testing.T) {
		text := "---\ntitle: T\ncount: 3\ndate: 20260922\n---\n\nbody\n"
		meta, body, err := ParseFrontmatter(text)
		if err != nil {
			t.Fatalf("ParseFrontmatter error: %v", err)
		}
		if meta == nil {
			t.Fatal("metadata = nil, want non-nil")
		}
		if meta.GetString("title") != "T" {
			t.Errorf("title = %q, want %q", meta.GetString("title"), "T")
		}
		if meta.GetInt("count") != 3 {
			t.Errorf("count = %d, want 3", meta.GetInt("count"))
		}
		if meta.GetType() != "yaml" {
			t.Errorf("type = %q, want yaml", meta.GetType())
		}
		if strings.TrimSpace(body) != "body" {
			t.Errorf("body = %q, want %q", body, "\nbody\n")
		}
	})

	t.Run("toml document", func(t *testing.T) {
		text := "+++\ntitle = 'T'\ncount = 3\n+++\n\nbody\n"
		meta, body, err := ParseFrontmatter(text)
		if err != nil {
			t.Fatalf("ParseFrontmatter error: %v", err)
		}
		if meta == nil {
			t.Fatal("metadata = nil, want non-nil")
		}
		if meta.GetString("title") != "T" || meta.GetInt("count") != 3 {
			t.Errorf("metadata = %v, want title T and count 3", meta.Data())
		}
		if meta.GetType() != "toml" {
			t.Errorf("type = %q, want toml", meta.GetType())
		}
		if strings.TrimSpace(body) != "body" {
			t.Errorf("body = %q, want %q", body, "\nbody\n")
		}
	})

	t.Run("no frontmatter returns ErrNoFrontmatter", func(t *testing.T) {
		text := "# Heading\n\nbody\n"
		meta, body, err := ParseFrontmatter(text)
		if !errors.Is(err, ErrNoFrontmatter) {
			t.Fatalf("err = %v, want ErrNoFrontmatter", err)
		}
		if meta != nil {
			t.Errorf("metadata = %v, want nil", meta)
		}
		if body != text {
			t.Errorf("body = %q, want the whole text", body)
		}
	})

	t.Run("empty block returns nil metadata without error", func(t *testing.T) {
		text := "---\n---\nbody\n"
		meta, body, err := ParseFrontmatter(text)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if meta != nil {
			t.Errorf("metadata = %v, want nil", meta)
		}
		if body != "body\n" {
			t.Errorf("body = %q, want %q", body, "body\n")
		}
	})

	t.Run("broken yaml line carries document line", func(t *testing.T) {
		text := "---\ntitle: T\nbroken: [1,\n---\n\nbody\n"
		_, _, err := ParseFrontmatter(text)
		var fmErr *FrontmatterError
		if !errors.As(err, &fmErr) {
			t.Fatalf("err = %v of type %T, want *FrontmatterError", err, err)
		}
		if fmErr.Line != 3 {
			t.Errorf("Line = %d, want 3 (the broken scalar line)", fmErr.Line)
		}
	})

	t.Run("broken toml carries document line", func(t *testing.T) {
		text := "+++\ntitle = 'T\n+++\n\nbody\n"
		_, _, err := ParseFrontmatter(text)
		var fmErr *FrontmatterError
		if !errors.As(err, &fmErr) {
			t.Fatalf("err = %v of type %T, want *FrontmatterError", err, err)
		}
		if fmErr.Line != 2 {
			t.Errorf("Line = %d, want 2", fmErr.Line)
		}
	})

	t.Run("non-mapping frontmatter is a FrontmatterError", func(t *testing.T) {
		text := "---\nplain scalar\n---\n\nbody\n"
		_, _, err := ParseFrontmatter(text)
		var fmErr *FrontmatterError
		if !errors.As(err, &fmErr) {
			t.Fatalf("err = %v of type %T, want *FrontmatterError", err, err)
		}
	})
}

func TestParseFrontmatterMatchesGetMetadata(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"yaml", "---\ntitle: T\ncount: 3\n---\n\nbody\n"},
		{"toml", "+++\ntitle = 'T'\ncount = 3\n+++\n\nbody\n"},
		{"no frontmatter", "# Heading\n\nbody\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, body, err := ParseFrontmatter(tt.text)
			if tt.name == "no frontmatter" && !errors.Is(err, ErrNoFrontmatter) {
				t.Fatalf("err = %v, want ErrNoFrontmatter", err)
			}
			if tt.name != "no frontmatter" && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}

			f := NewMDFileFromString(tt.text)
			fmeta, ferr := f.GetMetadata()
			if ferr != nil {
				t.Fatalf("GetMetadata error: %v", ferr)
			}

			if (fmeta == nil) != (meta == nil) {
				t.Fatalf("metadata nil mismatch: ParseFrontmatter %v, GetMetadata %v", meta, fmeta)
			}
			if meta != nil && !reflect.DeepEqual(meta.Data(), fmeta.Data()) {
				t.Errorf("metadata = %v, want %v", meta.Data(), fmeta.Data())
			}

			content, cerr := f.GetContent()
			if cerr != nil {
				t.Fatalf("GetContent error: %v", cerr)
			}
			if strings.TrimRight(body, "\n") != strings.TrimRight(content.RawBody, "\n") {
				t.Errorf("body = %q, want %q", body, content.RawBody)
			}
		})
	}
}
