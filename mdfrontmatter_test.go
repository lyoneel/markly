package markly

import (
	"reflect"
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
