package markly

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func fileNames(files []*MDFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.path
	}
	return out
}

// randomGraph returns a graph of n files with up to k dependencies
// each. cyclic allows edges to any node (cycles and self-loops);
// missing adds dependencies on files that do not exist; duplicates
// repeats some dependencies.
func randomGraph(r *rand.Rand, n, k int, cyclic, missing bool) map[string][]string {
	adj := make(map[string][]string, n)
	for i := range n {
		var deps []string
		for range r.IntN(k + 1) {
			switch {
			case missing && r.IntN(10) == 0:
				deps = append(deps, fmt.Sprintf("missing%d.md", r.IntN(3)))
			case cyclic:
				deps = append(deps, nodeName(r.IntN(n)))
			case i > 0:
				deps = append(deps, nodeName(r.IntN(i)))
			}
		}
		if len(deps) > 0 && r.IntN(4) == 0 {
			deps = append(deps, deps[0]) // duplicate dependency
		}
		adj[nodeName(i)] = deps
	}
	return adj
}

// realFolder is graphFolder without the placeholder files graphFolder
// adds for dependency targets, so missing targets stay missing.
func realFolder(adj map[string][]string) *MDFolder {
	mf := &MDFolder{files: make(map[string]*MDFile), adjacencyMap: make(map[string][]string), errors: make(map[string]error)}
	for name, deps := range adj {
		mf.files[name] = &MDFile{path: name}
		mf.adjacencyMap[name] = uniqueDeps(deps)
	}
	return mf
}

// The linear-time algorithms return exactly what the v1.8.0 ones did.
func TestGraphAlgorithmsMatchReference(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	for iter := range 400 {
		n := 1 + r.IntN(40)
		cyclic := iter%2 == 1
		missing := iter%3 == 2
		mf := realFolder(randomGraph(r, n, 4, cyclic, missing))
		label := fmt.Sprintf("iter %d (n=%d cyclic=%v missing=%v)", iter, n, cyclic, missing)

		adjCopy := make(map[string][]string)
		for k, v := range mf.adjacencyMap {
			adjCopy[k] = append([]string(nil), v...)
		}
		wantOrder, wantErr := refLoadOrder(mf, adjCopy)
		gotOrder, gotErr := mf.GetLoadOrder()
		if (wantErr == nil) != (gotErr == nil) || !reflect.DeepEqual(fileNames(wantOrder), fileNames(gotOrder)) {
			t.Fatalf("%s: GetLoadOrder = %v, %v; want %v, %v", label, fileNames(gotOrder), gotErr, fileNames(wantOrder), wantErr)
		}

		if want, got := refDetectCycles(mf), mf.DetectCycles(); !reflect.DeepEqual(want, got) {
			t.Fatalf("%s: DetectCycles = %q, want %q", label, got, want)
		}
		if gotErr == nil && len(mf.DetectCycles()) != 0 {
			t.Fatalf("%s: sorted, yet DetectCycles reports cycles", label)
		}

		for _, start := range []string{nodeName(r.IntN(n)), nodeName(n - 1), "nope.md"} {
			want, wErr := refAllWithDeps(mf, start, nil)
			got, gErr := mf.GetAllWithDependencies(start, nil)
			if fmt.Sprint(wErr) != fmt.Sprint(gErr) || !reflect.DeepEqual(fileNames(want), fileNames(got)) {
				t.Fatalf("%s: GetAllWithDependencies(%s) = %v, %v; want %v, %v", label, start, fileNames(got), gErr, fileNames(want), wErr)
			}
		}
	}
}

func TestDetectCyclesShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		adj  map[string][]string
		want []string
	}{
		{"self loop", map[string][]string{"a.md": {"a.md"}}, []string{"a.md"}},
		{"two nodes", map[string][]string{"a.md": {"b.md"}, "b.md": {"a.md"}}, []string{"a.md -> b.md"}},
		{"three nodes", map[string][]string{"a.md": {"b.md"}, "b.md": {"c.md"}, "c.md": {"a.md"}}, []string{"a.md -> b.md -> c.md"}},
		{"two separate", map[string][]string{"a.md": {"b.md"}, "b.md": {"a.md"}, "c.md": {"c.md"}}, []string{"a.md -> b.md", "c.md"}},
		{"duplicate edge", map[string][]string{"a.md": {"b.md", "b.md"}, "b.md": {"a.md"}}, []string{"a.md -> b.md"}},
		{"dag", map[string][]string{"a.md": {"b.md"}, "b.md": nil}, nil},
	} {
		mf := realFolder(tc.adj)
		for range 5 { // same result every run
			if got := mf.DetectCycles(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s: DetectCycles = %q, want %q", tc.name, got, tc.want)
			}
		}
		_, err := mf.GetLoadOrder()
		if (err != nil) != (tc.want != nil) {
			t.Fatalf("%s: GetLoadOrder err = %v", tc.name, err)
		}
	}
}

func TestGraphLargeDeterministic(t *testing.T) {
	const n = 5_000
	mf := graphFolder(randomDAG(n, 3))
	first, err := mf.GetLoadOrder()
	if err != nil || len(first) != n {
		t.Fatalf("GetLoadOrder: %d files, %v", len(first), err)
	}
	pos := make(map[string]int, n)
	for i, f := range first {
		pos[f.path] = i
	}
	for name, deps := range mf.adjacencyMap {
		for _, d := range deps {
			if pos[d] >= pos[name] {
				t.Fatalf("%s placed before its dependency %s", name, d)
			}
		}
	}
	for range 3 {
		again, _ := mf.GetLoadOrder()
		if !reflect.DeepEqual(fileNames(first), fileNames(again)) {
			t.Fatal("GetLoadOrder differs between runs")
		}
	}

	// A 100,000-file chain resolves without deep recursion.
	ch := graphFolder(chain(100_000))
	deps, err := ch.GetAllWithDependencies(nodeName(99_999), nil)
	if err != nil || len(deps) != 100_000 || deps[0].path != nodeName(0) {
		t.Fatalf("chain: %d files, %v", len(deps), err)
	}
	if c := ch.DetectCycles(); len(c) != 0 {
		t.Fatalf("chain DetectCycles = %v", c)
	}
	lad := graphFolder(ladderCycles(10_000))
	if got := len(lad.DetectCycles()); got != 9_999 {
		t.Fatalf("ladder cycles = %d, want 9999", got)
	}
	if !strings.Contains(lad.DetectCycles()[0], " -> ") {
		t.Fatal("ladder cycle format")
	}
}

func TestGetAllWithDependenciesCallerPath(t *testing.T) {
	mf := realFolder(map[string][]string{"a.md": {"b.md"}, "b.md": nil})
	if _, err := mf.GetAllWithDependencies("a.md", map[string]bool{"b.md": true}); err == nil ||
		!strings.Contains(err.Error(), "circular dependency detected at b.md") {
		t.Fatalf("caller path err = %v", err)
	}
	if _, err := mf.GetAllWithDependencies("a.md", map[string]bool{"a.md": true}); err == nil {
		t.Fatal("root on the caller path must be a cycle")
	}
}
