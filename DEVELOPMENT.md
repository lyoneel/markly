# markly Developer Reference

Developer guide for the markly package: architecture, full type documentation, dependency graph internals, and extension points. The README holds the overview and usage; this document holds the depth.

> Main repository: https://gitlab.com/lyoneel/markly
> Any other host that serves this repository is a mirror. Open issues
> and merge requests on GitLab.

## Architecture

The package is a single root package with these source files:

| File | Responsibility |
|------|----------------|
| `mdfile.go` | The `MDFile` type: constructors, lazy and eager loading, save paths |
| `mdmetadata.go` | The `MDMetadata` typed accessor |
| `mdcontent.go` | The `MDContent` and heading containers |
| `mdheadings.go` | ATX heading detection with line ranges |
| `mdbody.go` | Body line editing with absolute line numbers |
| `mdcheckbox.go` | Checkbox marker and title rewrites |
| `mdsections.go` | Section lookup by slug and section mutations |
| `mdconstants.go` | Shared constants and default keys |
| `mdfolder.go` | The `MDFolder` directory loader and dependency graph |
| `*_test.go` | Parsing, metadata, folder, and boundary test suites |

Design patterns:

- Lazy loading by default. Constructors parse metadata only; content loads on first `LoadContent()` or `GetContent()` call.
- Functional options on constructors and the folder loader (`WithRawScalars`, `WithFenceAwareHeadings`, `WithDepKeys`, `WithDepSeparator`, `WithSkipDotDirs`).
- Errors are values. Metadata parse failures in a folder are collected, not raised.

Dependency model:

- markly depends on dirly for file discovery and on `gopkg.in/yaml.v3` and `github.com/pelletier/go-toml/v2` for frontmatter parsing.

## Core Types

### MDFile

Represents a single Markdown file with YAML/TOML frontmatter.

**Key Features:**
- Automatic YAML/TOML frontmatter extraction and format detection
- Lazy content loading (content loaded only when explicitly requested)
- Heading detection and line range tracking
- Typed metadata access via `MDMetadata`
- Format type exposed via `Type` field ("yaml", "toml", or empty if no frontmatter)

**Usage:**

```go
// Create MDFile (lazy loading enabled by default - metadata parsed, content not loaded)
md := markly.NewMDFile("path/to/file.md")

// Access format type immediately after creation
formatType := md.Type  // "yaml", "toml", or "" if no frontmatter

// Access metadata (triggers YAML/TOML parsing only)
meta := md.GetMetadata()
title := meta.GetString("title")
tags := meta.GetStringList("tags")
active := meta.GetBool("active")

// Explicitly load content when needed
content, err := md.LoadContent()
if err != nil {
    log.Fatal(err)
}
headings := content.Headings
rawBody := content.RawBody

// Eager loading - load everything immediately
mdWithContent, err := markly.NewMDFileWithContent("path/to/file.md")
if err != nil {
    log.Fatal(err)
}
content := mdWithContent.GetContent() // Never returns error after successful creation
```

**Methods:**

| Method | Description |
|--------|-------------|
| `NewMDFile(path string) *MDFile` | Create with lazy loading enabled (metadata parsed, content not loaded) |
| `NewMDFileWithContent(path string) (*MDFile, error)` | Eager load metadata and content immediately |
| `SetLazyLoading(enabled bool)` | Toggle lazy loading behavior |
| `GetMetadata() *MDMetadata` | Access parsed YAML metadata (nil if file doesn't exist) |
| `GetContent() (*MDContent, error)` | Get parsed content - returns error if lazy loading enabled and not loaded |
| `LoadContent() error` | Explicitly trigger content loading |
| `GetContentFrom() int` | Line number where content starts (after YAML frontmatter), 0 if not loaded |

### MDMetadata

Typed accessor for YAML/TOML frontmatter data stored as `map[string]any`. Includes line range information and format type.

**Fields:**
- `FromLine int` - Starting line number of frontmatter content (after opening delimiter)
- `ToLine int` - Ending line number of frontmatter content (before closing delimiter)
- `Type string` - Format type: "yaml", "toml", or empty if no frontmatter

**Usage:**

```go
md := markly.NewMDFile("file.md")
meta := md.GetMetadata()

// Get format type
format := meta.GetType()  // "yaml" or "toml"

// Type-specific accessors (return zero value if not found/wrong type)
title := meta.GetString("title")     // Returns "" if not found/not a string
count := meta.GetInt("count")        // Returns 0 if not found/not a number
active := meta.GetBool("active")     // Returns false if not found/not a bool
tags := meta.GetStringList("tags")   // Returns nil if not found/not a list of strings

// Nested structures
nested := meta.GetMap("config")      // Returns nested map[string]any
rawList := meta.GetList("items")     // Returns []any (any type)

// Generic access
rawValue := meta.Get("key")          // Raw interface{} value for custom handling
keys := meta.Keys()                  // All top-level keys as []string

// Custom struct unmarshaling with yaml.v3 tags
var config MyConfig
err := meta.UnmarshalInto(&config)   // Uses yaml.v3 for optimal performance

// Line range tracking
fmt.Printf("YAML from line %d to %d\n", meta.FromLine, meta.ToLine)
```

**Format Detection:**
The package automatically detects the format based on the opening delimiter:
- `---` → YAML format (`md.Type = "yaml"`, `meta.GetType() == "yaml"`)
- `+++` → TOML format (`md.Type = "toml"`, `meta.GetType() == "toml"`)

**Type Mapping (YAML → Go):**
- Scalars: `string`, `int`/`float64` (converted to int), `bool`
- Sequences: `[]any` or `[]string` (via GetStringList)
- Mappings: `map[string]any`

### MDContent

Parsed Markdown content with heading information.

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `Headings` | `[]*MDHeading` | List of all headings with line ranges |
| `RawBody` | `string` | Original markdown body text (after YAML frontmatter) |

### MDHeading

Represents a single ATX-style heading (`#`, `##`, etc.).

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `Level` | `int` | Heading level (1-6 per Markdown spec) |
| `Text` | `string` | Heading text content (trimmed, without markers) |
| `FromLine` | `int` | Starting line number in the file (1-based) |
| `ToLine` | `int` | Ending line number in the file (same as FromLine for ATX headings) |

**Usage:**

```go
content, err := md.LoadContent()
if err != nil {
    log.Fatal(err)
}

for _, heading := range content.Headings {
    fmt.Printf("%s at lines %d-%d\n", 
        strings.Repeat("#", heading.Level), 
        heading.FromLine, heading.ToLine)
    fmt.Printf("  Text: %s\n", heading.Text)
}
```

### MDFolder

Directory-based Markdown file loader with dependency resolution and batch operations. Uses `dirly` internally for efficient file discovery.

**Usage:**

```go
// Basic usage (default dependency keys and separator)
folder, err := markly.NewMDFolder("./skills")
if err != nil {
    log.Fatal(err)
}

// Get all files (lazy loading enabled, metadata parsed immediately)
files := folder.GetAll()
for _, md := range files {
    fmt.Printf("%s: %d headings\n", md.path, len(md.GetContent().Headings))
}

// Custom dependency configuration
folder, _ := markly.NewMDFolder(
    "./docs",
    markly.WithDepKeys("requires_on"),      // Use custom field names
    markly.WithDepSeparator("; "),          // Use semicolon separator
)

// Get files in correct dependency order (topological sort)
orderedFiles, err := folder.GetLoadOrder()
if err != nil {
    log.Fatal(err)  // Circular dependency detected
}

// Check for cycles before processing
cycles := folder.DetectCycles()
for _, cycle := range cycles {
    log.Printf("Circular dependency: %s", cycle)
}

// Load specific file with all dependencies (transitive closure)
skillWithDeps, err := folder.GetAllWithDependencies("my-skill.md", nil)
if err != nil {
    log.Fatal(err)
}

// Filter files by metadata key-value pairs
activeDocs := folder.GetByMetadata(map[string]any{
    "active": true,
    "type":   "guide",
})

// Callback-based iteration
folder.Iterate(func(md *markly.MDFile) {
    // Process each file
})

// Get all files as a map
allFiles := folder.GetAll() // Returns map[string]*MDFile
for path, md := range allFiles {
    fmt.Printf("%s: %d headings\n", path, len(md.GetContent().Headings))
}
```

**Public Methods:**

| Method | Description |
|--------|-------------|
| `NewMDFolder(dirPath string, opts ...LoaderOption) (*MDFolder, error)` | Create loader with directory path and options |
| `GetAll() map[string]*MDFile` | All discovered files as map (metadata parsed, content lazy-loaded) |
| `Iterate(fn func(*MDFile))` | Callback-based iteration over all files |
| `GetByMetadata(filters map[string]any) []*MDFile` | Filter by metadata key-value pairs |
| `GetLoadOrder() ([]*MDFile, error)` | Topological sort of dependencies (Kahn's algorithm) |
| `DetectCycles() []string` | Return cycle paths as formatted strings (e.g., "A -> B -> A") |
| `GetAllWithDependencies(filename string, visited map[string]bool) ([]*MDFile, error)` | Transitive dependency resolution |

**LoaderOptions:**

```go
// Override default dependency field names
markly.WithDepKeys("my_custom_key", "another_key")

// Custom separator for comma-delimited strings (default: ",")
markly.WithDepSeparator("; ")
```

## Dependency Graph Internals

The MDFolder builds a directed acyclic graph (DAG) from metadata dependencies.

Files can declare dependencies using configurable metadata fields:

**Array format:**
```yaml
depends_on: ["basics.md", "concurrency.md"]
```

**String format (comma-separated):**
```yaml
prerequisites: "basics.md, concurrency.md"
```

With custom separator:
```yaml
requires: "file1.md; file2.md"  // With markly.WithDepSeparator("; ")
```

### Dependency Resolution Process

1. **Discovery**: Walk directory and load metadata for all `.md` files (content NOT loaded)
2. **Graph Building**: Extract dependencies from metadata, build adjacency map
3. **Cycle Detection**: Check for circular dependencies using DFS
4. **Topological Sort**: Use Kahn's algorithm to determine correct load order

### Performance Characteristics

| Phase | Memory Usage | Description |
|-------|--------------|-------------|
| Discovery | Minimal | File paths only (via dirly) |
| Graph Build | Low (~1-2KB/file) | Metadata only, no content loaded |
| Content Load | Variable | On-demand when accessing `GetContent()` or `LoadContent()` |

Without lazy loading, you'd load all content during graph building. With it, memory stays efficient until content is actually needed.

## Extension Points

- Register a frontmatter serializer with `SetSerializer` to control YAML key order, list style, and quoting. The hook applies to YAML documents only; TOML keeps the builtin marshaler.
- Add a new metadata accessor to `MDMetadata` following the `GetString` pattern: zero value on absence, no error return.
- Add loader options to `MDFolder` through the existing functional-options pattern.
- New frontmatter formats need a delimiter pair, a detection rule in the format detector, and a parse path in `mdfile.go`.
