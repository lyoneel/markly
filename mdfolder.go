package markly

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/lyoneel/dirly"
)

// MDFolder represents a directory of Markdown files with optional dependency resolution.
type MDFolder struct {
	dir          *dirly.Directory    // Private field - wrapped, not embedded
	files        map[string]*MDFile  // key = filename without extension
	adjacencyMap map[string][]string // "A" -> ["B", "C"] means A depends on B, C
	errors       map[string]error    // per-file metadata errors from the last load

	// Customizable dependency parsing configuration (private)
	depKeys      []string // Field names to check (e.g., ["depends", "dependencies"])
	depSeparator string   // Separator for comma-separated strings (e.g., ",")

	// Default file permissions - use dirly's defaults, no custom control needed
	defaultPerm os.FileMode // 0644 by default

	skipDotDirs bool // skip files inside dot-directories
}

// LoaderOption configures MDFolder behavior.
type LoaderOption func(*MDFolder)

// WithDepKeys sets custom field names to look for dependencies in metadata.
func WithDepKeys(keys ...string) LoaderOption {
	return func(mf *MDFolder) {
		if len(keys) > 0 {
			mf.depKeys = keys
		} else {
			mf.depKeys = []string{"depends", "dependencies", "depends_on", "prerequisites"}
		}
	}
}

// WithDepSeparator sets separator for parsing comma-delimited dependency strings.
func WithDepSeparator(sep string) LoaderOption {
	return func(mf *MDFolder) {
		if sep != "" {
			mf.depSeparator = sep
		} else {
			mf.depSeparator = ","
		}
	}
}

// WithSkipDotDirs skips files inside dot-directories (any path
// component starting with a dot) during discovery.
func WithSkipDotDirs(skip bool) LoaderOption {
	return func(mf *MDFolder) {
		mf.skipDotDirs = skip
	}
}

// NewMDFolder creates a new MDFolder for basic file discovery with default dependency config.
func NewMDFolder(dirPath string, opts ...LoaderOption) (*MDFolder, error) {
	mf := &MDFolder{
		files:        make(map[string]*MDFile),
		adjacencyMap: make(map[string][]string),
		errors:       make(map[string]error),
		depKeys:      []string{"deps", "depends", "dependencies", "depends_on", "prerequisites"},
		depSeparator: ",",
		defaultPerm:  0644, // Standard file permissions
	}

	// Initialize dirly.Directory with markdown extension filter (case-insensitive)
	mf.dir = dirly.NewFilteredDirectory(dirPath).WithExtensions("md").Build()

	for _, opt := range opts {
		opt(mf)
	}

	return mf, nil
}

// buildAdjacencyMap extracts dependencies from metadata and builds the dependency graph.
func (mf *MDFolder) buildAdjacencyMap() {
	for name, md := range mf.files {
		var deps []string

		meta, _ := md.GetMetadata()
		if meta == nil {
			continue // No YAML frontmatter, no dependencies
		}

		for _, key := range mf.depKeys {
			if list := meta.GetStringList(key); len(list) > 0 {
				for _, dep := range list {
					deps = append(deps, dep)
				}
			} else if strVal := meta.GetString(key); strVal != "" {
				parts := strings.Split(strVal, mf.depSeparator)
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						deps = append(deps, trimmed)
					}
				}
			}
		}

		mf.adjacencyMap[name] = deps
	}
}

// GetAll returns all discovered MDFiles (lazy loading enabled).
func (mf *MDFolder) GetAll() map[string]*MDFile {
	return mf.GetAllByGlob("*")
}

// pathHasDotDir reports whether any path component starts with a dot.
func pathHasDotDir(relPath string) bool {
	for _, component := range strings.Split(filepath.ToSlash(relPath), "/") {
		if strings.HasPrefix(component, ".") && component != "." {
			return true
		}
	}
	return false
}

// Errors returns the per-file metadata errors collected during the
// last discovery pass, keyed by relative path.
func (mf *MDFolder) Errors() map[string]error {
	return mf.errors
}

// GetAllByGlob returns all discovered MDFiles (lazy loading enabled).
func (mf *MDFolder) GetAllByGlob(glob string) map[string]*MDFile {
	relPaths, err := mf.dir.GetAllByGlobRel(glob)
	if err != nil {
		return nil
	}

	result := make(map[string]*MDFile)
	for _, path := range relPaths {
		if mf.skipDotDirs && pathHasDotDir(path) {
			continue
		}
		fullPath := filepath.Join(mf.dir.BasePath(), path)
		md := NewMDFile(fullPath)

		// Process metadata immediately to ensure it's available
		if err = md.processMetadata(); err != nil && !os.IsNotExist(err) {
			mf.errors[path] = err
			continue // Skip files with metadata errors
		}

		result[path] = md
		mf.files[path] = md
	}

	mf.buildAdjacencyMap()

	return result
}

// Iterate invokes fn for each file in arbitrary order.
func (mf *MDFolder) Iterate(fn func(*MDFile)) {
	if len(mf.files) == 0 {
		mf.GetAll() // Ensure files are loaded first
	}
	for _, md := range mf.files {
		fn(md)
	}
}

// GetByMetadata returns files matching all key-value pairs.
func (mf *MDFolder) GetByMetadata(filters map[string]any) []*MDFile {
	if len(mf.files) == 0 {
		mf.GetAll() // Ensure files are loaded first
	}
	var result []*MDFile

	for _, md := range mf.files {
		meta, _ := md.GetMetadata()
		if meta == nil {
			continue
		}

		match := true
		for key, expected := range filters {
			switch v := expected.(type) {
			case string:
				if meta.GetString(key) != v {
					match = false
				}
			case int:
				if meta.GetInt(key) != v {
					match = false
				}
			case bool:
				if meta.GetBool(key) != v {
					match = false
				}
			default:
				match = false
			}

			if !match {
				break
			}
		}

		if match {
			result = append(result, md)
		}
	}

	return result
}

// getLoadOrderInternal is a helper that performs topological sort on a copy of the adjacency map.
func (mf *MDFolder) getLoadOrderInternal(adjMapCopy map[string][]string) ([]*MDFile, error) {
	ordered := make([]*MDFile, 0, len(mf.files))
	inDegree := make(map[string]int)

	for file := range mf.files {
		if _, exists := inDegree[file]; !exists {
			inDegree[file] = 0
		}
	}

	for file, deps := range adjMapCopy {
		for _, dep := range deps {
			if _, exists := inDegree[dep]; !exists {
				inDegree[dep] = 0
			}
			inDegree[file]++ // file depends on dep
		}
	}

	queue := make([]string, 0)
	for file, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, file)
		}
	}

	// Make another copy to avoid modifying during sort
	tempAdjMap := make(map[string][]string)
	for k, v := range adjMapCopy {
		tempAdjMap[k] = make([]string, len(v))
		copy(tempAdjMap[k], v)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if md, exists := mf.files[current]; exists {
			ordered = append(ordered, md)
		}

		// Reduce in-degree for files that depend on current
		for file, deps := range tempAdjMap {
			for i, dep := range deps {
				if dep == current {
					inDegree[file]--
					if inDegree[file] == 0 {
						queue = append(queue, file)
					}
					tempAdjMap[file] = append(deps[:i], deps[i+1:]...)
					break
				}
			}
		}
	}

	if len(ordered) != len(mf.files) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return ordered, nil
}

// GetLoadOrder returns files in topological order (dependencies first).
func (mf *MDFolder) GetLoadOrder() ([]*MDFile, error) {
	if len(mf.files) == 0 {
		mf.GetAll() // Ensure files are loaded first
	}

	// Create a copy of adjacency map to avoid modifying original
	adjMapCopy := make(map[string][]string)
	for k, v := range mf.adjacencyMap {
		adjMapCopy[k] = make([]string, len(v))
		copy(adjMapCopy[k], v)
	}

	return mf.getLoadOrderInternal(adjMapCopy)
}

// DetectCycles returns all cycle paths as []string.
func (mf *MDFolder) DetectCycles() []string {
	if len(mf.files) == 0 {
		mf.GetAll() // Ensure files are loaded first
	}

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

	for file := range mf.files {
		if !visited[file] {
			dfs(file)
		}
	}

	return cycles
}

// GetAllWithDependencies returns a file and all its transitive dependencies in order.
// Uses separate path/processed maps: path for cycle detection, processed for deduplication.
func (mf *MDFolder) GetAllWithDependencies(filename string, visited map[string]bool) ([]*MDFile, error) {
	if visited == nil {
		if len(mf.files) == 0 {
			mf.GetAll()
		}
		visited = make(map[string]bool)
	}

	// visited serves as the DFS path for cycle detection
	if visited[filename] {
		return nil, fmt.Errorf("circular dependency detected at %s", filename)
	}

	visited[filename] = true

	// Use internal method with separate processed cache
	return mf.getAllWithDepsInternal(filename, visited, make(map[string]bool))
}

// getAllWithDepsInternal is the internal recursive implementation.
// path: nodes in current DFS stack (cycle detection)
// processed: already-resolved files (deduplication)
func (mf *MDFolder) getAllWithDepsInternal(filename string, path, processed map[string]bool) ([]*MDFile, error) {
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

		deps, err := mf.getAllWithDepsInternal(dep, path, processed)
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
