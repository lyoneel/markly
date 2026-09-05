# Markly Package

Markdown file parsing and management with YAML and TOML frontmatter support.

> Main repository: https://gitlab.com/lyoneel/markly
> Any other host that serves this repository is a mirror. Open issues
> and merge requests on GitLab.

## Overview

The `markly` package provides tools for reading, parsing, and managing Markdown files with YAML or TOML frontmatter metadata. It supports lazy loading, dependency resolution between files, typed metadata access, and efficient batch operations using the `dirly` integration.

**Key Features:**
- Automatic YAML/TOML frontmatter extraction with line number tracking and format detection
- Lazy content loading (content loaded only when explicitly requested)
- ATX-style heading detection with line range information
- Typed metadata access via `MDMetadata`
- Directory-based file discovery with dependency graph resolution
- Batch read/write operations for efficiency

## Supported Frontmatter Formats

The package supports both YAML and TOML frontmatter formats:

| Format | Opening Delimiter | Closing Delimiter | Example |
|--------|-------------------|-------------------|---------|
| YAML   | `---`            | `---`            | See below |
| TOML   | `+++`            | `+++`            | See below |

### YAML Frontmatter

```yaml
---
name: my-doc
tags:
  - guide
  - tutorial
active: true
---

# Document Title
```

### TOML Frontmatter

```toml
+++
name = "my-doc"
tags = ["guide", "tutorial"]
active = true
+++

# Document Title
```

**Note:** The package automatically detects the format based on the opening delimiter. Mixed delimiters in code blocks are handled correctly - only the first frontmatter section is parsed as metadata.

## Core Types

| Type | Purpose |
|------|---------|
| `MDFile` | A single Markdown file with lazy or eager loading, frontmatter parsing, and body editing |
| `MDMetadata` | Typed accessor for frontmatter data with line ranges and format type |
| `MDContent` | Parsed body with heading list and raw text |
| `MDHeading` | One ATX heading with its line range |
| `MDFolder` | Directory loader with dependency resolution, filtering, and batch operations |

Basic usage:

```go
// Lazy: metadata parsed, content not loaded
md := markly.NewMDFile("path/to/file.md")
meta := md.GetMetadata()
title := meta.GetString("title")

// Load content on demand
content, err := md.LoadContent()

// Eager: load everything immediately
md, err := markly.NewMDFileWithContent("path/to/file.md")
```

The complete type documentation, method tables, and loader options live in [DEVELOPMENT.md](DEVELOPMENT.md). The generated reference lives on pkg.go.dev.

## Dependency Graph

The MDFolder builds a directed acyclic graph (DAG) from metadata dependencies.

Files declare dependencies in frontmatter, in array or string form:

```yaml
depends_on: ["basics.md", "concurrency.md"]
```

```yaml
prerequisites: "basics.md, concurrency.md"
```

With a custom separator:

```yaml
requires: "file1.md; file2.md"  // With markly.WithDepSeparator("; ")
```

Resolution runs in four phases: discovery (paths only), graph building (metadata only), cycle detection (DFS), and topological sort (Kahn's algorithm). Content loads lazily on first access. The full resolution process and performance characteristics live in [DEVELOPMENT.md](DEVELOPMENT.md).

## Default Dependency Keys

By default, the package checks these field names for dependencies (in order):
1. `deps` (short form)
2. `depends`
3. `dependencies`
4. `depends_on`
5. `prerequisites`

Override with `WithDepKeys()`:

```go
NewMDFolder(path, markly.WithDepKeys("my_custom_key"))
```

## Line Number Tracking

All parsing preserves line numbers for error reporting and navigation:

```go
md := markly.NewMDFile("file.md")

// YAML frontmatter line range
meta := md.GetMetadata()
if meta != nil {
    fmt.Printf("YAML from line %d to %d\n", meta.FromLine, meta.ToLine)
}

// Content loading and heading tracking
content, err := md.LoadContent()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Content starts at line %d\n", md.GetContentFrom())

for _, heading := range content.Headings {
    fmt.Printf("%s at lines %d-%d: %s\n", 
        strings.Repeat("#", heading.Level),
        heading.FromLine, heading.ToLine,
        heading.Text)
}
```

## Error Handling

All operations return errors for proper error handling:

| Operation | Possible Errors |
|-----------|-----------------|
| `NewMDFileWithContent()` | File not found (`os.IsNotExist`), parse errors |
| `GetContent()` / `LoadContent()` | File not found, YAML parse errors, scanner errors (e.g., token too long) |
| `GetMetadata()` | Returns nil if file doesn't exist or has no YAML frontmatter |
| `GetLoadOrder()` | Circular dependency detected |
| `GetAllWithDependencies()` | Circular dependency, file not found |

**Error Patterns:**

```go
md := markly.NewMDFile("nonexistent.md")
meta := md.GetMetadata()  // Returns nil (no error)

content, err := md.LoadContent()
if err != nil {
    if os.IsNotExist(err) {
        // Handle missing file
    } else if strings.Contains(err.Error(), "token too long") {
        // Handle scanner error for very long lines
    }
}

// Eager loading returns error immediately
md, err := markly.NewMDFileWithContent("file.md")
if err != nil {
    log.Fatal(err)
}
```

## Document Manipulation

Beyond parsing, markly can construct documents in memory and mutate
their frontmatter and body.

### In-Memory Construction

```go
md := markly.NewMDFileFromString(content)        // YAML/TOML + body parsed immediately
md := markly.NewMDFileFromBytes(data)            // same, from bytes

meta, _ := md.GetMetadata()
meta.SetString("title", "New Title")

md.SetPath("/vault/note.md")                     // give it a write target
if err := md.Save(); err != nil {                // direct write
    return err
}
if err := md.SaveAtomic(); err != nil {          // temp file + rename
    return err
}
```

`Save()` keeps the classic direct write. `SaveAtomic()` writes a
uniquely named temp file in the target directory and renames it over
the target, so a crash mid-write leaves the previous file intact.

### Loading Options

All constructors accept options:

```go
md := markly.NewMDFileFromString(content,
    markly.WithRawScalars(),          // keep dates/timestamps as original text
    markly.WithFenceAwareHeadings(),  // skip headings inside ``` / ~~~ fences
)
```

`WithRawScalars()` converts `!!bool`, `!!int`, `!!float`, `!!null`
scalars to their natural types but keeps everything else (dates,
timestamps, plain strings) as the exact original text, so round-trips
never rewrite values.

### Metadata Structure Inspection

```go
meta, _ := md.GetMetadata()

data := meta.Data()                       // underlying map[string]any
meta.IsFlowSequence("tags")               // tags: [a, b] -> true
meta.HasBlockScalarSequence()             // tags:\n  - a -> true
```

### Pluggable Frontmatter Serializer

YAML serialization defaults to `yaml.Marshal`. Register a hook to
control key order, list style, and quoting:

```go
md.SetSerializer(func(meta map[string]any) (string, error) {
    // canonical example: sorted keys, hash last
    keys := make([]string, 0, len(meta))
    for k := range meta {
        if k != "hash" {
            keys = append(keys, k)
        }
    }
    sort.Strings(keys)

    var b strings.Builder
    for _, k := range keys {
        fmt.Fprintf(&b, "%s: %v\n", k, meta[k])
    }
    if h, ok := meta["hash"]; ok {
        fmt.Fprintf(&b, "hash: %v\n", h)
    }
    return b.String(), nil
})
```

The hook applies to YAML documents only; TOML keeps the builtin
marshaler.

### Body Editing

Body mutations use absolute 1-based file line numbers, so stored
anchors point at the same lines a human sees in the file.

```go
md.AppendLine("- [ ] new item")          // append to body
md.InsertLine(5, "inserted line")        // insert before file line 5
md.RemoveLine(7)                         // remove file line 7
md.SetBody("fresh\nbody")                // replace the whole body
```

### First-Level Headings

```go
title, found := md.ExtractFirstH1()      // first "# Title" outside code fences
md.RemoveFirstH1()                       // strip it from the body
md.SetFirstHeading("New Title")          // replace, or insert after frontmatter
```

### Sections

```go
line, found := md.FindSection("my-section")      // slug lookup, file line number
md.SetSectionHeading("my-section", "Renamed")    // rename "## My Section"
md.SetSectionBullet("my-section", "status", "done")  // replace/insert "- status: done"
md.InsertLineUnderHeading("My Section", "- [ ] moved", true)  // true: create if missing
```

`SlugHeading("My Section!")` produces the stable slug `my-section`
(lowercase, non-alphanumeric runs collapsed to one hyphen).

### Checkboxes

```go
md.SetCheckboxMarker(5, 'x')             // "- [ ] task" -> "- [x] task"
md.RewriteCheckboxTitle(5, "renamed")    // keeps indent, marker, (due: ...),
                                         // and capture-timestamp suffixes
```

### Heading Slug Helper

```go
slug := markly.SlugHeading("Hello World!") // "hello-world"
```

## Folder Discovery Options

```go
folder, _ := markly.NewMDFolder("./vault",
    markly.WithSkipDotDirs(true),   // skip files inside dot-directories
)

files := folder.GetAll()
for path, err := range folder.Errors() {
    // files skipped because their frontmatter failed to parse
    fmt.Printf("%s: %v\n", path, err)
}
```

Metadata parse errors no longer log; they are collected and exposed
through `Errors()`, keyed by relative path.

## Testing

Run tests with race detection:

```bash
go test -race ./...
```

The package includes comprehensive tests covering:
- YAML frontmatter parsing (various formats, edge cases)
- Heading detection and extraction (ATX-style, all levels 1-6)
- Lazy vs eager loading behavior
- Dependency graph building and resolution
- Cycle detection in complex DAGs
- Metadata filtering by key-value pairs
- Typed accessor methods (GetString, GetInt, GetBool, etc.)
- Batch read/write operations
- Line number tracking accuracy
- Real-world scenarios (kaizen structures, boolean fields, mixed arrays)

## Example: Complete Workflow

```go
package main

import (
    "fmt"
    "log"
    
    "gitlab.com/lyoneel/markly"
)

func main() {
    // Load all markdown files from directory
    folder, err := markly.NewMDFolder("./docs")
    if err != nil {
        log.Fatal(err)
    }

    // Get files in dependency order
    orderedFiles, err := folder.GetLoadOrder()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Processing %d files in dependency order:\n", len(orderedFiles))
    
    for _, md := range orderedFiles {
        meta := md.GetMetadata()
        
        // Extract metadata
        title := meta.GetString("title")
        tags := meta.GetStringList("tags")
        
        fmt.Printf("\n%s\n", title)

        if len(tags) > 0 {
            fmt.Printf("Tags: %s\n", tags)
        }
        
        // Load content (lazy - only when needed)
        content, err := md.LoadContent()
        if err != nil {
            log.Printf("Error loading %s: %v\n", md.path, err)
            continue
        }
        
        fmt.Printf("Headings: %d\n", len(content.Headings))
        for _, h := range content.Headings {
            fmt.Printf("  - [%d] %s (lines %d-%d)\n", 
                h.Level, h.Text, h.FromLine, h.ToLine)
        }
    }
}
```

## Comparison: Lazy vs Eager Loading

| Aspect | Lazy Loading (`NewMDFile`) | Eager Loading (`NewMDFileWithContent`) |
|--------|---------------------------|---------------------------------------|
| Metadata parsing | Yes (on `GetMetadata()` call) | Yes (immediate) |
| Content loading | No (explicit via `LoadContent()`) | Yes (immediate) |
| First access cost | Low (metadata only) | High (full file parse) |
| Memory usage | Minimal until content loaded | Full file in memory immediately |
| Use case | Directory scanning, metadata filtering | Single-file processing, immediate content needs |

**Recommendation:** Use lazy loading for directory operations (`MDFolder`), eager loading when you need content immediately.

## License

MIT License - See [LICENSE](LICENSE) file for details.
