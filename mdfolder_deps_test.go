package markly

import (
	"path/filepath"
	"strings"
	"testing"
)

func depsFolder(t *testing.T, files map[string]string) *MDFolder {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeFile(t, filepath.Join(dir, name), content)
	}
	f, err := NewMDFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.GetAll()
	return f
}

func orderNames(files []*MDFile) []string {
	var names []string
	for _, md := range files {
		names = append(names, filepath.Base(md.path))
	}
	return names
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

func TestDuplicateDependencyIsNotACycle(t *testing.T) {
	f := depsFolder(t, map[string]string{
		"a.md": "---\ntitle: a\n---\n",
		"b.md": "---\ndepends: [a.md, a.md]\n---\n",
		"c.md": "---\ndepends: [a.md, b.md, a.md]\n---\n",
	})
	order, err := f.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder: %v", err)
	}
	names := orderNames(order)
	if len(names) != 3 || indexOf(names, "a.md") > indexOf(names, "b.md") || indexOf(names, "b.md") > indexOf(names, "c.md") {
		t.Fatalf("order = %v", names)
	}
	if cycles := f.DetectCycles(); len(cycles) != 0 {
		t.Fatalf("DetectCycles = %v", cycles)
	}
	deps, err := f.GetAllWithDependencies("c.md", nil)
	if err != nil || strings.Join(orderNames(deps), ",") != "a.md,b.md,c.md" {
		t.Fatalf("GetAllWithDependencies = %v, %v", orderNames(deps), err)
	}
}

func TestDiamondDependencies(t *testing.T) {
	f := depsFolder(t, map[string]string{
		"base.md":  "---\ntitle: base\n---\n",
		"left.md":  "---\ndepends: [base.md]\n---\n",
		"right.md": "---\ndepends: [base.md]\n---\n",
		"top.md":   "---\ndepends: [left.md, right.md, base.md]\n---\n",
	})
	order, err := f.GetLoadOrder()
	if err != nil {
		t.Fatalf("GetLoadOrder: %v", err)
	}
	names := orderNames(order)
	if names[0] != "base.md" || names[3] != "top.md" {
		t.Fatalf("order = %v", names)
	}
	deps, err := f.GetAllWithDependencies("top.md", nil)
	if err != nil || len(deps) != 4 {
		t.Fatalf("GetAllWithDependencies = %v, %v", orderNames(deps), err)
	}
}

func TestRealCyclesStillDetected(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"two-node": {
			"a.md": "---\ndepends: [b.md, b.md]\n---\n",
			"b.md": "---\ndepends: [a.md]\n---\n",
		},
		"self": {
			"a.md": "---\ndepends: [a.md, a.md]\n---\n",
		},
		"three-node": {
			"a.md": "---\ndepends: [b.md]\n---\n",
			"b.md": "---\ndepends: [c.md]\n---\n",
			"c.md": "---\ndepends: [a.md]\n---\n",
		},
	} {
		f := depsFolder(t, files)
		if _, err := f.GetLoadOrder(); err == nil || !strings.Contains(err.Error(), "circular") {
			t.Fatalf("%s: GetLoadOrder = %v; want circular dependency", name, err)
		}
		if len(f.DetectCycles()) == 0 {
			t.Fatalf("%s: DetectCycles found nothing", name)
		}
	}
}

func TestLoadOrderIsStable(t *testing.T) {
	files := map[string]string{
		"a.md": "---\ntitle: a\n---\n",
		"b.md": "---\ntitle: b\n---\n",
		"c.md": "---\ndepends: [a.md]\n---\n",
		"d.md": "---\ntitle: d\n---\n",
	}
	want := ""
	for i := 0; i < 5; i++ {
		order, err := depsFolder(t, files).GetLoadOrder()
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(orderNames(order), ",")
		if i == 0 {
			want = got
		} else if got != want {
			t.Fatalf("order changed between runs: %s vs %s", got, want)
		}
	}
	if want != "a.md,b.md,d.md,c.md" {
		t.Fatalf("order = %s", want)
	}
}
