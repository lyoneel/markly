package markly

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// aliasBomb builds a "billion laughs" frontmatter: each level holds ten
// aliases of the previous one, so levels+1 levels expand to 10^(levels+1)
// scalars.
func aliasBomb(levels int) string {
	var b strings.Builder
	b.WriteString("---\na: &a [x,x,x,x,x,x,x,x,x,x]\n")
	prev := "a"
	for i := 0; i < levels; i++ {
		n := string(rune('b' + i))
		fmt.Fprintf(&b, "%s: &%s [*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s]\n", n, n, prev, prev, prev, prev, prev, prev, prev, prev, prev, prev)
		prev = n
	}
	b.WriteString("---\nbody\n")
	return b.String()
}

func TestRawScalarsRejectsAliasBomb(t *testing.T) {
	for _, levels := range []int{6, 7, 9} {
		start := time.Now()
		md := NewMDFileFromString(aliasBomb(levels), WithRawScalars())
		meta, err := md.GetMetadata()
		if !errors.Is(err, ErrExcessiveAliasing) {
			t.Fatalf("levels=%d: GetMetadata = %v, %v; want ErrExcessiveAliasing", levels, meta, err)
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("levels=%d: rejection took %v", levels, d)
		}
	}
}

func TestRawScalarsAliasBombFromDisk(t *testing.T) {
	path := t.TempDir() + "/bomb.md"
	writeFile(t, path, aliasBomb(7))
	_, err := NewMDFileWithContent(path, WithRawScalars())
	if !errors.Is(err, ErrExcessiveAliasing) {
		t.Fatalf("NewMDFileWithContent = %v; want ErrExcessiveAliasing", err)
	}
}

func TestRawScalarsKeepsNormalAliases(t *testing.T) {
	doc := "---\nbase: &base\n  owner: ana\n  created: 2026-01-02\nlist: &l [a, b, c]\nx: *base\ny: *base\ntags: *l\ncount: &n 3\ntotal: *n\n---\nbody\n"
	md := NewMDFileFromString(doc, WithRawScalars())
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	x, ok := meta.Get("x").(map[string]any)
	if !ok || x["owner"] != "ana" || x["created"] != "2026-01-02" {
		t.Fatalf("x = %#v", meta.Get("x"))
	}
	if tags, ok := meta.Get("tags").([]any); !ok || len(tags) != 3 || tags[2] != "c" {
		t.Fatalf("tags = %#v", meta.Get("tags"))
	}
	if meta.Get("total") != int64(3) {
		t.Fatalf("total = %#v", meta.Get("total"))
	}
	// Expanded aliases are independent copies.
	x["owner"] = "changed"
	if y := meta.Get("y").(map[string]any); y["owner"] != "ana" {
		t.Fatalf("y shares storage with x: %#v", y)
	}
}

func TestRawScalarsAllowsModerateAliasReuse(t *testing.T) {
	// A 50-item anchor reused 100 times expands to 5,000 values: well
	// inside the budget.
	var b strings.Builder
	b.WriteString("---\nitems: &i [")
	for i := 0; i < 50; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "v%d", i)
	}
	b.WriteString("]\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "k%d: *i\n", i)
	}
	b.WriteString("---\n")
	meta, err := NewMDFileFromString(b.String(), WithRawScalars()).GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if got := meta.Get("k99").([]any); len(got) != 50 {
		t.Fatalf("k99 has %d items", len(got))
	}
}

func TestDefaultParseStillRejectsAliasBomb(t *testing.T) {
	_, err := NewMDFileFromString(aliasBomb(7)).GetMetadata()
	if err == nil {
		t.Fatal("default parse accepted an alias bomb")
	}
}
