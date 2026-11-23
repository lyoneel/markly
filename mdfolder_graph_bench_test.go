package markly

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// graphFolder builds an in-memory folder over the given adjacency, so
// graph benchmarks and tests measure the algorithms, not disk reads.
func graphFolder(adj map[string][]string) *MDFolder {
	mf := &MDFolder{files: make(map[string]*MDFile), adjacencyMap: make(map[string][]string), errors: make(map[string]error)}
	for name, deps := range adj {
		mf.files[name] = &MDFile{path: name}
		mf.adjacencyMap[name] = uniqueDeps(deps)
		for _, d := range deps {
			if _, ok := mf.files[d]; !ok {
				mf.files[d] = &MDFile{path: d}
			}
		}
	}
	return mf
}

func nodeName(i int) string { return fmt.Sprintf("n%06d.md", i) }

// randomDAG: node i depends on up to k random earlier nodes.
func randomDAG(n, k int) map[string][]string {
	r := rand.New(rand.NewPCG(1, 2))
	adj := make(map[string][]string, n)
	for i := range n {
		var deps []string
		for j := 0; j < k && i > 0; j++ {
			deps = append(deps, nodeName(r.IntN(i)))
		}
		adj[nodeName(i)] = deps
	}
	return adj
}

// chain: node i depends on node i-1, the deepest possible closure.
func chain(n int) map[string][]string {
	adj := make(map[string][]string, n)
	adj[nodeName(0)] = nil
	for i := 1; i < n; i++ {
		adj[nodeName(i)] = []string{nodeName(i - 1)}
	}
	return adj
}

// ladderCycles: node i depends on i+1 and i-1, so every edge back up
// the DFS path closes a two-node cycle deep in the path.
func ladderCycles(n int) map[string][]string {
	adj := make(map[string][]string, n)
	for i := range n {
		var deps []string
		if i+1 < n {
			deps = append(deps, nodeName(i+1))
		}
		if i > 0 {
			deps = append(deps, nodeName(i-1))
		}
		adj[nodeName(i)] = deps
	}
	return adj
}

func BenchmarkGraph(b *testing.B) {
	for _, n := range []int{1_000, 10_000} {
		dag := graphFolder(randomDAG(n, 3))
		b.Run(fmt.Sprintf("GetLoadOrder/random/%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := dag.GetLoadOrder(); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("DetectCycles/random/%d", n), func(b *testing.B) {
			for b.Loop() {
				if c := dag.DetectCycles(); len(c) != 0 {
					b.Fatal(c)
				}
			}
		})
		top := nodeName(n - 1)
		b.Run(fmt.Sprintf("GetAllWithDependencies/random/%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := dag.GetAllWithDependencies(top, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
		ch := graphFolder(chain(n))
		b.Run(fmt.Sprintf("GetAllWithDependencies/chain/%d", n), func(b *testing.B) {
			for b.Loop() {
				if deps, err := ch.GetAllWithDependencies(top, nil); err != nil || len(deps) != n {
					b.Fatal(len(deps), err)
				}
			}
		})
		lad := graphFolder(ladderCycles(n))
		b.Run(fmt.Sprintf("DetectCycles/ladder/%d", n), func(b *testing.B) {
			for b.Loop() {
				if c := lad.DetectCycles(); len(c) == 0 {
					b.Fatal("no cycles")
				}
			}
		})
	}
}
