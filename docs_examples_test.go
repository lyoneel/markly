package markly_test

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"strings"
	"testing"
)

// docBlock is one ```go fence from a Markdown document.
type docBlock struct {
	file string
	line int
	code string
}

func goBlocks(t *testing.T, file string) []docBlock {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var blocks []docBlock
	var cur *docBlock
	var b strings.Builder
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		switch {
		case cur == nil && strings.TrimSpace(line) == "```go":
			cur = &docBlock{file: file, line: n + 1}
			b.Reset()
		case cur != nil && strings.TrimSpace(line) == "```":
			cur.code = b.String()
			blocks = append(blocks, *cur)
			cur = nil
		case cur != nil:
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return blocks
}

// docPrelude wraps a snippet in a function whose parameters stand for
// the values the prose introduces (md, meta, folder, text, ...). The
// snippet sits in an inner block so it may redeclare them with :=.
const docPrelude = `package docs

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"gitlab.com/lyoneel/markly"
	"gitlab.com/lyoneel/markly/validate"
)

var (
	_ = bufio.ErrTooLong
	_ = errors.New
	_ = fmt.Sprint
	_ = log.Print
	_ = os.Stat
	_ = sort.Strings
	_ = strings.Repeat
)

func _(md *markly.MDFile, meta *markly.MDMetadata, folder *markly.MDFolder,
	schema *validate.Schema, text string, content string, raw []byte,
	data map[string]any, myIndex any) error {
	_, _, _, _, _, _, _, _, _ = md, meta, folder, schema, text, content, raw, data, myIndex
	{
`

// ignoredDocErrors are type errors that only mean a snippet is a
// fragment: values shown but not used, or no trailing return.
var ignoredDocErrors = []string{"declared and not used", "is not used", "missing return"}

// TestDocExamplesCompile type-checks every ```go block in README.md and
// DEVELOPMENT.md against the current API, so the examples cannot drift.
func TestDocExamplesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the module from source")
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "source", nil)
	for _, doc := range []string{"README.md", "DEVELOPMENT.md"} {
		blocks := goBlocks(t, doc)
		if len(blocks) == 0 {
			t.Fatalf("%s: no go blocks found", doc)
		}
		for _, blk := range blocks {
			name := fmt.Sprintf("%s:%d", blk.file, blk.line)
			src := blk.code
			offset := blk.line - 1
			if !strings.HasPrefix(strings.TrimSpace(src), "package ") {
				src = docPrelude + src + "\t}\n}\n"
				offset = blk.line - 1 - strings.Count(docPrelude, "\n")
			}
			f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
			if err != nil {
				t.Errorf("%s: parse: %v", name, err)
				continue
			}
			var errs []string
			conf := types.Config{Importer: imp, Error: func(err error) {
				te := err.(types.Error)
				for _, s := range ignoredDocErrors {
					if strings.Contains(te.Msg, s) {
						return
					}
				}
				pos := fset.Position(te.Pos)
				errs = append(errs, fmt.Sprintf("%s:%d: %s", blk.file, pos.Line+offset, te.Msg))
			}}
			_, _ = conf.Check("docs", fset, []*ast.File{f}, nil)
			for _, e := range errs {
				t.Error(e)
			}
		}
	}
}
