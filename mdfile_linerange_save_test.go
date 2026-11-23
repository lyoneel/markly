package markly

import (
	"path/filepath"
	"testing"
)

func TestLineRangeAfterSaveMatchesParser(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		edit func(*MDMetadata)
	}{
		{"yaml", "---\na: 1\nb: 2\n---\nbody\n", func(m *MDMetadata) { m.SetInt("a", 3) }},
		{"yaml grows", "---\na: 1\n---\nbody\n", func(m *MDMetadata) { m.SetString("b", "x"); m.SetString("c", "y") }},
		{"toml", "+++\na = 1\nb = 2\n+++\nbody\n", func(m *MDMetadata) { m.SetInt("a", 3) }},
	} {
		for _, atomic := range []bool{false, true} {
			dir := t.TempDir()
			md := NewMDFileFromString(tc.doc)
			m, err := md.GetMetadata()
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(m)
			md.SetPath(filepath.Join(dir, "x.md"))
			if atomic {
				err = md.SaveAtomic()
			} else {
				err = md.Save()
			}
			if err != nil {
				t.Fatal(err)
			}
			// Reparse the saved file: the in-memory range must equal it.
			re, err := NewMDFile(filepath.Join(dir, "x.md")).GetMetadata()
			if err != nil {
				t.Fatal(err)
			}
			if m.FromLine != re.FromLine || m.ToLine != re.ToLine {
				t.Fatalf("%s atomic=%v: after save %d-%d, reparsed %d-%d", tc.name, atomic, m.FromLine, m.ToLine, re.FromLine, re.ToLine)
			}
			if m.FromLine != 2 {
				t.Fatalf("%s: FromLine = %d, want 2", tc.name, m.FromLine)
			}
		}
	}
}
