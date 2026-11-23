package markly

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/lyoneel/dirly"
)

// MDFolder represents a directory of Markdown files with optional dependency resolution.
type MDFolder struct {
	dir          *dirly.Directory    // Private field - wrapped, not embedded
	files        map[string]*MDFile  // key = slash path relative to the folder, with extension ("sub/a.md")
	adjacencyMap map[string][]string // "A" -> ["B", "C"] means A depends on B, C
	errors       map[string]error    // per-file metadata errors from the last load

	// Customizable dependency parsing configuration (private)
	depKeys      []string // Field names to check (e.g., ["depends", "dependencies"])
	depSeparator string   // Separator for comma-separated strings (e.g., ",")

	skipDotDirs bool // skip files inside dot-directories

	symlinkPolicy SymlinkPolicy // how symlinks below the root are treated

	fileOpts []FileOption // applied to every file the folder loads
}

// LoaderOption configures MDFolder behavior.
type LoaderOption func(*MDFolder)

// WithFolderMaxFileSize sets the largest file, in bytes, the folder
// reads; it is WithMaxFileSize applied to every file the folder loads.
// A larger file is skipped and reported in Errors() with an error
// wrapping ErrFileTooLarge, and a lazy LoadContent of a file that grew
// past the limit fails the same way. The default is DefaultMaxFileSize;
// a limit of 0 or less disables the check.
func WithFolderMaxFileSize(limit int64) LoaderOption {
	return func(mf *MDFolder) {
		mf.fileOpts = append(mf.fileOpts, WithMaxFileSize(limit))
	}
}

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

// WithDepSeparator sets the separator that splits a dependency field
// written as one string ("a.md, b.md"). Entries are trimmed, and empty
// entries and repeats are dropped. An empty sep keeps the default ",".
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

// WithSymlinkPolicy sets how symlinks below the folder root are treated
// when files are read: FollowWithinRoot (the default), FollowAny, or
// NoFollow. Files refused by the policy are skipped and reported through
// Errors. Use FollowAny to keep the behavior of markly v1.7.1 and
// earlier, where a symlinked file was read wherever it pointed. An
// unknown value makes NewMDFolder fail with ErrInvalidSymlinkPolicy.
func WithSymlinkPolicy(policy SymlinkPolicy) LoaderOption {
	return func(mf *MDFolder) {
		mf.symlinkPolicy = policy
	}
}

// ErrNotADirectory reports a NewMDFolder path that exists but is not a
// directory.
var ErrNotADirectory = errors.New("not a directory")

// NewMDFolder creates a new MDFolder for basic file discovery with default dependency config.
// The path must be an existing directory (or a symlink to one); a
// missing path returns the os.Stat error, so os.IsNotExist works, and a
// file returns an error wrapping ErrNotADirectory.
// Files are read through dirly under the folder's symlink policy
// (FollowWithinRoot unless WithSymlinkPolicy says otherwise).
func NewMDFolder(dirPath string, opts ...LoaderOption) (*MDFolder, error) {
	mf := &MDFolder{
		files:        make(map[string]*MDFile),
		adjacencyMap: make(map[string][]string),
		errors:       make(map[string]error),
		depKeys:      []string{"deps", "depends", "dependencies", "depends_on", "prerequisites"},
		depSeparator: ",",

		symlinkPolicy: DefaultSymlinkPolicy,
	}

	for _, opt := range opts {
		opt(mf)
	}
	if !validSymlinkPolicy(mf.symlinkPolicy) {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSymlinkPolicy, mf.symlinkPolicy)
	}

	// The folder must exist and be a directory (a symlink to one is
	// fine). The error is the *fs.PathError from os.Stat, so
	// os.IsNotExist and errors.Is(err, os.ErrNotExist) work.
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &fs.PathError{Op: "open folder", Path: dirPath, Err: ErrNotADirectory}
	}

	// Initialize dirly.Directory with markdown extension filter (case-insensitive).
	// Options run first so the symlink policy is part of the build.
	mf.dir = dirly.NewFilteredDirectory(dirPath).
		WithExtensions("md").
		WithSymlinkPolicy(mf.symlinkPolicy).
		Build()

	return mf, nil
}

// SymlinkPolicy returns the symlink policy the folder reads files with.
func (mf *MDFolder) SymlinkPolicy() SymlinkPolicy {
	return mf.dir.SymlinkPolicy()
}

// fileOpener returns an opener that reads relPath through dirly's
// Directory.Open, so the folder's symlink policy applies to every read.
// Under FollowWithinRoot and NoFollow dirly opens the file through an
// os.Root at the resolved folder root, which also holds if a path
// component is swapped for a symlink after discovery.
func (mf *MDFolder) fileOpener(relPath string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return mf.dir.Open(relPath)
	}
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
			deps = append(deps, depValues(meta, key, mf.depSeparator)...)
		}

		mf.adjacencyMap[name] = uniqueDeps(deps)
	}
}

// depValues reads the dependencies declared under key. A string value
// is a separator-delimited list: it is split on sep (the default ","
// when sep is empty), each entry is trimmed, and empty entries are
// dropped, so "a.md, b.md" and "a.md" both work. A YAML list keeps its
// string items as written. Before, GetStringList wrapped any string in a
// one-item list, so the split never ran and "a.md, b.md" was one
// dependency named "a.md, b.md".
func depValues(meta *MDMetadata, key, sep string) []string {
	str, isString := meta.Get(key).(string)
	if !isString {
		return meta.GetStringList(key)
	}
	if sep == "" {
		sep = ","
	}
	var deps []string
	for _, part := range strings.Split(str, sep) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			deps = append(deps, trimmed)
		}
	}
	return deps
}

// uniqueDeps drops repeated dependency entries, keeping the first
// occurrence, so listing a file twice is one edge, not two.
func uniqueDeps(deps []string) []string {
	if len(deps) < 2 {
		return deps
	}
	seen := make(map[string]bool, len(deps))
	out := deps[:0:0]
	for _, d := range deps {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// GetAll returns all discovered MDFiles (lazy loading enabled). Like
// GetAllByGlob, it replaces the state of the previous load.
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
// last discovery pass, keyed by relative path. A failure of discovery
// itself (for example the folder became unreadable) is reported under
// the empty key "".
func (mf *MDFolder) Errors() map[string]error {
	return mf.errors
}

// GetAllByGlob returns all discovered MDFiles matching glob (lazy
// loading enabled). Each call is a fresh load: the files, dependency
// graph and Errors() of the folder are replaced by the result of this
// pass, so Iterate, GetByMetadata, GetLoadOrder and the dependency
// helpers see exactly the files it returned.
func (mf *MDFolder) GetAllByGlob(glob string) map[string]*MDFile {
	// Each load replaces the folder state: files removed since the
	// previous load disappear, and errors that were fixed are cleared.
	mf.files = make(map[string]*MDFile)
	mf.errors = make(map[string]error)
	mf.adjacencyMap = make(map[string][]string)

	result := make(map[string]*MDFile)
	relPaths, err := mf.dir.GetAllByGlobRel(glob)
	if err != nil {
		mf.errors[""] = fmt.Errorf("discover files: %w", err)
		return result
	}

	for _, path := range relPaths {
		if mf.skipDotDirs && pathHasDotDir(path) {
			continue
		}
		fullPath := filepath.Join(mf.dir.BasePath(), path)
		md := NewMDFile(fullPath, mf.fileOpts...)
		md.open = mf.fileOpener(path)
		md.write = mf.fileWriter(path)
		md.writePath = fullPath

		// Process metadata immediately to ensure it's available
		if err = md.processMetadata(); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // removed since discovery
			}
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

// depGraph is the dependency graph with nodes numbered in name order,
// so every walk over it is deterministic without sorting as it goes.
// Nodes are the folder's files plus any dependency target that is not
// a file; edges point from a file to its distinct dependencies.
type depGraph struct {
	names  []string // node names, sorted
	deps   [][]int  // deps[i]: dependencies of node i, in declaration order
	isFile []bool   // node is a loaded file, not only a dependency target
}

// buildGraph indexes the adjacency map. Cost: one sort of the node
// names, then linear in nodes plus edges.
func (mf *MDFolder) buildGraph() *depGraph {
	seen := make(map[string]bool, len(mf.files))
	names := make([]string, 0, len(mf.files))
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for name := range mf.files {
		add(name)
	}
	for name, deps := range mf.adjacencyMap {
		add(name)
		for _, d := range deps {
			add(d)
		}
	}
	sort.Strings(names)
	index := make(map[string]int, len(names))
	for i, n := range names {
		index[n] = i
	}
	edges := 0
	for _, deps := range mf.adjacencyMap {
		edges += len(deps)
	}
	// All dependency lists share one backing array.
	flat := make([]int, 0, edges)
	g := &depGraph{names: names, deps: make([][]int, len(names)), isFile: make([]bool, len(names))}
	// mark[d] == i+1 when node i already lists d: a repeated dependency
	// counts once without a per-node set.
	mark := make([]int, len(names))
	for i, n := range names {
		_, g.isFile[i] = mf.files[n]
		start := len(flat)
		for _, name := range mf.adjacencyMap[n] {
			if d := index[name]; mark[d] != i+1 {
				mark[d] = i + 1
				flat = append(flat, d)
			}
		}
		g.deps[i] = flat[start:len(flat):len(flat)]
	}
	return g
}

// GetLoadOrder returns files in topological order (dependencies first).
//
// It runs Kahn's algorithm in time linear in files plus dependencies
// (after one sort of the names). Files with no dependency between them
// come out in a fixed order: the files without dependencies in name
// order, then each file as soon as its last dependency is placed, in
// name order among files freed by the same dependency. The result is
// the same on every run. A cycle returns an error.
func (mf *MDFolder) GetLoadOrder() ([]*MDFile, error) {
	if len(mf.files) == 0 {
		mf.GetAll() // Ensure files are loaded first
	}
	g := mf.buildGraph()
	n := len(g.names)

	// dependents[d] lists the nodes depending on d in ascending order,
	// because they are appended while i ascends; so the nodes one
	// placement frees are already in name order.
	// The lists are laid out in one array (counting sort by dependency).
	inDegree := make([]int, n)
	offset := make([]int, n+1)
	for i, deps := range g.deps {
		inDegree[i] = len(deps)
		for _, d := range deps {
			offset[d+1]++
		}
	}
	for d := range n {
		offset[d+1] += offset[d]
	}
	flat := make([]int, offset[n])
	fill := append([]int(nil), offset[:n]...)
	for i, deps := range g.deps {
		for _, d := range deps {
			flat[fill[d]] = i
			fill[d]++
		}
	}
	dependents := func(d int) []int { return flat[offset[d]:offset[d+1]] }

	queue := make([]int, 0, n)
	for i := range n {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	ordered := make([]*MDFile, 0, len(mf.files))
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		if g.isFile[current] {
			ordered = append(ordered, mf.files[g.names[current]])
		}
		for _, dep := range dependents(current) {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(ordered) != len(mf.files) {
		return nil, fmt.Errorf("circular dependency detected")
	}
	return ordered, nil
}

// DetectCycles returns cycle paths as strings such as "a.md -> b.md".
// A file that depends on itself is the one-file cycle "a.md".
//
// It is an iterative depth-first search, linear in files plus
// dependencies plus the length of the reported cycles, so deep chains
// cannot overflow the stack. The search starts from files in name
// order and follows dependencies in declaration order, so the result
// is the same on every run. Each edge that closes a cycle is reported
// once; a cycle that is only reachable through an already explored
// file is covered by the report for that file.
func (mf *MDFolder) DetectCycles() []string {
	if len(mf.files) == 0 {
		mf.GetAll() // Ensure files are loaded first
	}
	g := mf.buildGraph()
	n := len(g.names)

	const unvisited = -2
	const done = -1
	// state[i] is unvisited, done, or the position of node i on path.
	state := make([]int, n)
	for i := range state {
		state[i] = unvisited
	}
	next := make([]int, n) // next dependency of node i to follow
	var path []int
	var cycles []string

	for root := range n {
		if state[root] != unvisited || !g.isFile[root] {
			continue
		}
		state[root] = 0
		path = append(path[:0], root)
		for len(path) > 0 {
			node := path[len(path)-1]
			if next[node] == len(g.deps[node]) {
				state[node] = done
				path = path[:len(path)-1]
				continue
			}
			dep := g.deps[node][next[node]]
			next[node]++
			switch state[dep] {
			case unvisited:
				state[dep] = len(path)
				path = append(path, dep)
			case done:
			default: // on the path: this edge closes a cycle
				cycle := make([]string, 0, len(path)-state[dep])
				for _, i := range path[state[dep]:] {
					cycle = append(cycle, g.names[i])
				}
				cycles = append(cycles, strings.Join(cycle, " -> "))
			}
		}
	}
	return cycles
}

// GetAllWithDependencies returns a file and all its transitive
// dependencies, each once, dependencies before the files that need
// them: a depth-first post-order that follows dependencies in
// declaration order. It runs in time linear in the files and
// dependencies it reaches.
//
// visited, when not nil, holds the files already on the caller's
// resolution path; reaching one of them is a cycle. A cycle returns an
// error naming the file that closes it, and a dependency that is not a
// file in the folder returns a "file not found" error.
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
	path := visited

	type frame struct {
		name string
		next int
	}
	processed := map[string]bool{filename: true}
	stack := []frame{{name: filename}}
	var result []*MDFile

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		deps := mf.adjacencyMap[top.name]
		if top.next < len(deps) {
			dep := deps[top.next]
			top.next++
			if path[dep] {
				return nil, fmt.Errorf("circular dependency detected at %s", dep)
			}
			if processed[dep] {
				// Already placed (it is not on the path, so it is done).
				if _, exists := mf.files[dep]; !exists {
					return nil, fmt.Errorf("file not found: %s", dep)
				}
				continue
			}
			processed[dep] = true
			path[dep] = true
			stack = append(stack, frame{name: dep})
			continue
		}

		md, exists := mf.files[top.name]
		if !exists {
			return nil, fmt.Errorf("file not found: %s", top.name)
		}
		result = append(result, md)
		if len(stack) > 1 {
			path[top.name] = false // backtrack; the root stays marked
		}
		stack = stack[:len(stack)-1]
	}
	return result, nil
}
