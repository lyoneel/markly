package markly

import (
	"path/filepath"
	"reflect"
	"testing"
)

func depsFolderWith(t *testing.T, files map[string]string, opts ...LoaderOption) *MDFolder {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeFile(t, filepath.Join(dir, name), content)
	}
	f, err := NewMDFolder(dir, opts...)
	if err != nil {
		t.Fatal(err)
	}
	f.GetAll()
	return f
}

func TestStringDependencyListsAreSplit(t *testing.T) {
	cases := []struct {
		name string
		meta string
		opts []LoaderOption
		want []string
	}{
		{"default comma", `prerequisites: "a.md, b.md"`, nil, []string{"a.md", "b.md"}},
		{"comma without spaces", `deps: "a.md,b.md,c.md"`, nil, []string{"a.md", "b.md", "c.md"}},
		{"single value", `depends: "a.md"`, nil, []string{"a.md"}},
		{"whitespace trimmed", "depends: \"  a.md ,\\tb.md  \"", nil, []string{"a.md", "b.md"}},
		{"empty entries dropped", `depends: ",a.md,, ,b.md,"`, nil, []string{"a.md", "b.md"}},
		{"only separators", `depends: " , ,"`, nil, nil},
		{"duplicates dropped", `depends: "a.md, b.md, a.md, b.md"`, nil, []string{"a.md", "b.md"}},
		{"custom separator", `requires: "a.md; b.md"`,
			[]LoaderOption{WithDepKeys("requires"), WithDepSeparator("; ")}, []string{"a.md", "b.md"}},
		{"custom separator keeps commas", `requires: "a,1.md|b.md"`,
			[]LoaderOption{WithDepKeys("requires"), WithDepSeparator("|")}, []string{"a,1.md", "b.md"}},
		{"empty separator means comma", `depends: "a.md, b.md"`,
			[]LoaderOption{WithDepSeparator("")}, []string{"a.md", "b.md"}},
		{"yaml flow list", `depends: [a.md, b.md]`, nil, []string{"a.md", "b.md"}},
		{"yaml block list", "depends:\n  - a.md\n  - b.md\n  - a.md", nil, []string{"a.md", "b.md"}},
		{"yaml list items are not split", `depends: ["a.md, b.md"]`, nil, []string{"a.md, b.md"}},
		{"several keys merged", "deps: \"a.md, b.md\"\ndepends: [b.md, c.md]", nil, []string{"a.md", "b.md", "c.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := depsFolderWith(t, map[string]string{
				"x.md": "---\n" + tc.meta + "\n---\nbody\n",
			}, tc.opts...)
			if errs := f.Errors(); len(errs) != 0 {
				t.Fatalf("Errors() = %v", errs)
			}
			if got := f.adjacencyMap["x.md"]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("deps = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStringDependencyListOrdersFiles(t *testing.T) {
	f := depsFolderWith(t, map[string]string{
		"a.md": "---\ntitle: a\n---\n",
		"b.md": "---\nprerequisites: \"a.md\"\n---\n",
		"c.md": "---\nprerequisites: \"b.md, a.md, b.md\"\n---\n",
	})
	order, err := f.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder: %v", err)
	}
	if got := orderNames(order); !reflect.DeepEqual(got, []string{"a.md", "b.md", "c.md"}) {
		t.Fatalf("order = %v", got)
	}
	if cycles := f.DetectCycles(); len(cycles) != 0 {
		t.Fatalf("DetectCycles = %v", cycles)
	}
	deps, err := f.GetAllWithDependencies("c.md", nil)
	if err != nil || len(deps) != 3 {
		t.Fatalf("GetAllWithDependencies = %v, %v", orderNames(deps), err)
	}
}

func TestStringDependencyCycleIsDetected(t *testing.T) {
	f := depsFolderWith(t, map[string]string{
		"a.md": "---\ndepends: \"b.md, c.md\"\n---\n",
		"b.md": "---\ndepends: \"a.md\"\n---\n",
		"c.md": "---\ntitle: c\n---\n",
	})
	if _, err := f.GetLoadOrder(); err == nil {
		t.Fatal("GetLoadOrder: want a cycle error")
	}
	if cycles := f.DetectCycles(); len(cycles) == 0 {
		t.Fatal("DetectCycles: want the a.md <-> b.md cycle")
	}
}
