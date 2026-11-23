package markly

// Reference implementations: the v1.8.0 graph algorithms, kept
// verbatim (DetectCycles walks roots in sorted order instead of map
// order) as oracles for the linear-time versions.

import (
	"fmt"
	"sort"
	"strings"
)

func sortedKeys(m map[string]*MDFile) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// getLoadOrderInternal performs a topological sort (Kahn's algorithm)
// over the adjacency map. Edges are counted once per distinct
// dependency, and a reverse index of dependents keeps the sort linear
// in files plus edges. Ties are broken by name so the order is stable.
func refLoadOrder(mf *MDFolder, adjMapCopy map[string][]string) ([]*MDFile, error) {
	ordered := make([]*MDFile, 0, len(mf.files))
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for file := range mf.files {
		inDegree[file] = 0
	}
	for file, deps := range adjMapCopy {
		if _, exists := inDegree[file]; !exists {
			inDegree[file] = 0
		}
		for _, dep := range uniqueDeps(deps) {
			if _, exists := inDegree[dep]; !exists {
				inDegree[dep] = 0
			}
			inDegree[file]++ // file depends on dep
			dependents[dep] = append(dependents[dep], file)
		}
	}

	queue := make([]string, 0)
	for file, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, file)
		}
	}
	sort.Strings(queue)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if md, exists := mf.files[current]; exists {
			ordered = append(ordered, md)
		}

		var ready []string
		for _, file := range dependents[current] {
			inDegree[file]--
			if inDegree[file] == 0 {
				ready = append(ready, file)
			}
		}
		sort.Strings(ready)
		queue = append(queue, ready...)
	}

	if len(ordered) != len(mf.files) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return ordered, nil
}

// DetectCycles returns all cycle paths as []string.
func refDetectCycles(mf *MDFolder) []string {
	var cycles []string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, dep := range mf.adjacencyMap[node] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := append([]string{}, path[cycleStart:]...)
					cycles = append(cycles, strings.Join(cycle, " -> "))
				}
			}
		}

		recStack[node] = false
		path = path[:len(path)-1]
		return false
	}

	for _, file := range sortedKeys(mf.files) {
		if !visited[file] {
			dfs(file)
		}
	}

	return cycles
}

// GetAllWithDependencies returns a file and all its transitive dependencies in order.
// Uses separate path/processed maps: path for cycle detection, processed for deduplication.
func refAllWithDeps(mf *MDFolder, filename string, visited map[string]bool) ([]*MDFile, error) {
	if visited == nil {
		visited = make(map[string]bool)
	}

	// visited serves as the DFS path for cycle detection
	if visited[filename] {
		return nil, fmt.Errorf("circular dependency detected at %s", filename)
	}

	visited[filename] = true

	// Use internal method with separate processed cache
	return refAllWithDepsInternal(mf, filename, visited, make(map[string]bool))
}

// getAllWithDepsInternal is the internal recursive implementation.
// path: nodes in current DFS stack (cycle detection)
// processed: already-resolved files (deduplication)
func refAllWithDepsInternal(mf *MDFolder, filename string, path, processed map[string]bool) ([]*MDFile, error) {
	// If already processed, return cached file
	if processed[filename] {
		if md, exists := mf.files[filename]; exists {
			return []*MDFile{md}, nil
		}
		return nil, fmt.Errorf("file not found: %s", filename)
	}

	processed[filename] = true
	var result []*MDFile

	for _, dep := range mf.adjacencyMap[filename] {
		// Cycle check: dep is in current DFS path
		if path[dep] {
			return nil, fmt.Errorf("circular dependency detected at %s", dep)
		}
		path[dep] = true

		deps, err := refAllWithDepsInternal(mf, dep, path, processed)
		if err != nil {
			return nil, err
		}
		result = append(result, deps...)

		path[dep] = false // backtrack
	}

	// Deduplicate by path, preserving order
	seen := make(map[string]bool)
	deduped := make([]*MDFile, 0, len(result))
	for _, md := range result {
		if !seen[md.path] {
			seen[md.path] = true
			deduped = append(deduped, md)
		}
	}
	result = deduped

	if md, exists := mf.files[filename]; exists {
		result = append(result, md)
	} else {
		return nil, fmt.Errorf("file not found: %s", filename)
	}

	return result, nil
}
