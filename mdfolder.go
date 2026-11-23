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

// getLoadOrderInternal performs a topological sort (Kahn's algorithm)
// over the adjacency map. Edges are counted once per distinct
// dependency, and a reverse index of dependents keeps the sort linear
// in files plus edges. Ties are broken by name so the order is stable.
func (mf *MDFolder) getLoadOrderInternal(adjMapCopy map[string][]string) ([]*MDFile, error) {
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
