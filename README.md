# Markly Package

Markdown file parsing and management with YAML and TOML frontmatter support.

> Main repository: https://gitlab.com/lyoneel/markly
> Any other host that serves this repository is a mirror. Open issues
> and merge requests on GitLab.

## Overview

The `markly` package provides tools for reading, parsing, and managing Markdown files with YAML or TOML frontmatter metadata. It supports lazy loading, dependency resolution between files, typed metadata access, and directory discovery through `dirly`.

**Key Features:**
- Automatic YAML/TOML frontmatter extraction with line number tracking and format detection
- Lazy content loading (content loaded only when explicitly requested)
- ATX-style heading detection with line range information
- Typed metadata access via `MDMetadata`
- Directory-based file discovery with dependency graph resolution

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

**Note:** The package detects the format from the opening delimiter. A frontmatter block must start on line 1 and close with the same delimiter it opened with. A document whose first line is anything else (including a blank line), whose block is never closed, or whose block mixes `---` and `+++` has no frontmatter, and the whole text is body. Only that first block is metadata; a later `---` (for example a horizontal rule) is body text.

## Core Types

| Type | Purpose |
|------|---------|
| `MDFile` | A single Markdown file with lazy or eager loading, frontmatter parsing, and body editing |
| `MDMetadata` | Typed accessor for frontmatter data with line ranges and format type |
| `MDContent` | Parsed body with heading list and raw text |
| `MDHeading` | One ATX heading with its line range |
| `MDFolder` | Directory loader with dependency resolution and metadata filtering |

Basic usage:

```go
// Lazy: metadata parsed, content not loaded
md := markly.NewMDFile("path/to/file.md")
meta, err := md.GetMetadata() // nil, nil when the file has no frontmatter
if err != nil {
    return err
}
if meta != nil {
    fmt.Println(meta.GetString("title"))
}

// Load content on demand, then read it
if err := md.LoadContent(); err != nil {
    return err
}
content, _ := md.GetContent()

// Eager: load everything immediately
eager, err := markly.NewMDFileWithContent("path/to/file.md")
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
# With markly.WithDepKeys("requires") and markly.WithDepSeparator("; ")
requires: "file1.md; file2.md"
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
folder, err := markly.NewMDFolder("./docs", markly.WithDepKeys("my_custom_key"))
```

## Line Number Tracking

All parsing preserves line numbers for error reporting and navigation:

```go
md := markly.NewMDFile("file.md")

// Frontmatter line range: the lines between the delimiters
meta, err := md.GetMetadata()
if err != nil {
    log.Fatal(err)
}
if meta != nil {
    fmt.Printf("%s from line %d to %d\n", meta.GetType(), meta.FromLine, meta.ToLine)
}

// Content loading and heading tracking
if err := md.LoadContent(); err != nil {
    log.Fatal(err)
}
content, _ := md.GetContent()

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
| `LoadContent()` | File not found (`os.IsNotExist`), parse errors, a line longer than `markly.MaxLineLength` (wraps `bufio.ErrTooLong`) |
| `GetContent()` | The `LoadContent()` errors; with lazy loading, an error until `LoadContent()` has run |
| `GetMetadata()` | File not found (`os.IsNotExist`) or unreadable, parse errors; `nil, nil` when the file has no frontmatter |
| `Save()` / `SaveAtomic()` / `Render()` | `markly.ErrMetadataSerialization` when the metadata cannot be serialized (the file is left untouched), write errors |
| Single-line edits (`AppendLine`, `InsertLine`, `SetSectionHeading`, `SetSectionBullet`, `InsertLineUnderHeading`, `SetFirstHeading`, `RewriteCheckboxTitle`) | `markly.ErrMultilineText` when the text contains `\n` or `\r` |
| `SetCheckboxMarker()` | `markly.ErrInvalidCheckboxMarker` for a marker other than `' '`, `'x'`, `'X'`, `'~'` |
| `WithRawScalars()` parsing | `markly.ErrExcessiveAliasing` when YAML aliases expand past the budget |
| `NewMDFolder()` | Folder not found (`os.IsNotExist`), `markly.ErrNotADirectory`, `markly.ErrInvalidSymlinkPolicy` |
| `GetLoadOrder()` | Circular dependency detected |
| `GetAllWithDependencies()` | Circular dependency, file not found |
| `validate.LoadSchema()` | `validate.ErrSchema`; `validate.ErrVocabularyEscape` for a `from` reference outside the schema directory |

`Serialize()` returns `""` when the metadata cannot be serialized; use
`Render()` to get the error.

**Error Patterns:**

```go
md := markly.NewMDFile("nonexistent.md")
meta, err := md.GetMetadata() // meta is nil, err is a *fs.PathError
if os.IsNotExist(err) {
    // Handle missing file
}

if err := md.LoadContent(); err != nil {
    if os.IsNotExist(err) {
        // Handle missing file
    } else if errors.Is(err, bufio.ErrTooLong) {
        // A line is longer than markly.MaxLineLength
    }
}

// Eager loading returns error immediately
eager, err := markly.NewMDFileWithContent("file.md")
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
fromBytes := markly.NewMDFileFromBytes(raw)      // same, from bytes

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
uniquely named temp file in the target directory, syncs it, renames it
over the target, and syncs the directory, so a crash mid-write leaves
the previous file intact. It keeps the existing file's mode (new files
get `markly.DefaultFilePerm`, 0644) and, where the platform allows,
its owner, and it writes through a symlink to the link's target
instead of replacing the link. Files loaded from an `MDFolder` save
through dirly's atomic writer under the folder's symlink policy; that
write fails with `dirly.ErrOwnership` if the file's owner cannot be
kept, leaving the file untouched.

Single-line edits reject text containing a line break with
`markly.ErrMultilineText`, so a title or bullet cannot inject extra
lines or a second frontmatter block.

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
Alias expansion is budgeted (10,000 values plus ten per YAML node in the
document), so an alias bomb fails fast with `markly.ErrExcessiveAliasing`
instead of exhausting memory.

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
body, found := md.FindSectionBody("my-section")  // slug lookup, section text
md.SetSectionHeading("my-section", "Renamed")    // rename "## My Section"
md.SetSectionBullet("my-section", "status", "done")  // replace/insert "- status: done"
md.InsertLineUnderHeading("My Section", "- [ ] moved", true)  // true: create if missing
```

`FindSectionBody` returns the section text up to the next heading of
the same or higher level, with leading and trailing blank lines
trimmed.

`SlugHeading("My Section!")` produces the stable slug `my-section`
(lowercase, non-alphanumeric runs collapsed to one hyphen).

### Frontmatter Parsing with Errors

`GetMetadata` treats every parse failure the same way (nil metadata).
When callers need to tell the failure modes apart, use the one-shot
parse:

```go
meta, body, err := markly.ParseFrontmatter(text)
if errors.Is(err, markly.ErrNoFrontmatter) {
    // no delimiter block at all
}
var fmErr *markly.FrontmatterError
if errors.As(err, &fmErr) {
    // fmErr.Line points at the document line that broke the parse
}
```

Coercing accessors read YAML int-typed scalars (dates) and decimal
strings without changing `GetString` semantics:

```go
meta.GetCoercedString("date")  // int scalar 20260922 -> "20260922"
meta.GetCoercedInt("count")    // string "42" -> 42
```

Strict duplicate-key detection is opt-in:

```go
md := markly.NewMDFileFromString(text, markly.WithStrictDuplicates())
// a repeated top-level key fails with *markly.FrontmatterError
```

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
    // files skipped because their frontmatter failed to parse,
    // or because the symlink policy refused them
    fmt.Printf("%s: %v\n", path, err)
}
```

Metadata parse errors no longer log; they are collected and exposed
through `Errors()`, keyed by relative path. A failure of discovery
itself is stored under the empty key `""`. Every `GetAll` or
`GetAllByGlob` call replaces the files, the dependency graph, and
`Errors()` with the result of that load, and a file deleted between
discovery and parsing is skipped. `NewMDFolder` fails when the folder
does not exist (`os.IsNotExist`) or is not a directory
(`markly.ErrNotADirectory`).

Dependencies listed more than once count once, and `GetLoadOrder`
returns files that have no ordering constraint between them in
lexical order, so the result is the same on every run.

### Symlinks

`MDFolder` reads every file through dirly under a symlink policy, set
with `WithSymlinkPolicy`:

| Policy | Behavior |
|--------|----------|
| `markly.FollowWithinRoot` (default) | Symlinks are followed while the target stays inside the folder. A file whose link resolves outside the folder is skipped and reported in `Errors()` with an error wrapping `markly.ErrPathEscapesRoot`. |
| `markly.FollowAny` | Symlinks are followed wherever they point (the behavior of v1.7.1 and earlier). |
| `markly.NoFollow` | Any file reached through a symlink below the folder is skipped with `markly.ErrSymlinkNotAllowed`. |

```go
folder, err := markly.NewMDFolder("./vault") // FollowWithinRoot by default
// markly.NewMDFolder("./vault", markly.WithSymlinkPolicy(markly.FollowAny))
// opts back in to links that point outside the folder.
if err != nil {
    return err
}
folder.GetAll()
for path, err := range folder.Errors() {
    if errors.Is(err, markly.ErrPathEscapesRoot) {
        fmt.Printf("%s: link points outside the folder\n", path)
    }
}
```

The policy is checked again on every lazy load, so a file swapped for an
escaping symlink after discovery is refused too. A folder path that is
itself a symlink (for example `~/notes -> ~/Sync/notes`) needs no
change: its real location becomes the boundary. An unknown policy value
makes `NewMDFolder` fail with `markly.ErrInvalidSymlinkPolicy`. Files
opened directly with `NewMDFile` or `NewMDFileWithContent` are not
affected by the policy. Folder files are also saved through dirly
under the policy (see In-Memory Construction).

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
- Line number tracking accuracy
- Real-world scenarios (workflow structures, boolean fields, mixed arrays)

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
        meta, err := md.GetMetadata()
        if err != nil {
            log.Printf("Error reading metadata: %v\n", err)
            continue
        }
        if meta == nil {
            continue // no frontmatter
        }

        // Extract metadata
        title := meta.GetString("title")
        tags := meta.GetStringList("tags")
        
        fmt.Printf("\n%s\n", title)

        if len(tags) > 0 {
            fmt.Printf("Tags: %s\n", tags)
        }
        
        // Load content (lazy - only when needed)
        if err := md.LoadContent(); err != nil {
            log.Printf("Error loading content: %v\n", err) // err names the file
            continue
        }
        content, _ := md.GetContent()
        
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

## Schema Validation

The `validate` sub-package checks frontmatter against a schema file: a
markdown document whose frontmatter carries a `fields` map and an
optional `config` map of format constants.

```yaml
fields:
  title: {required: true, shape: string}
  status: {required: true, shape: single, values: [draft, done]}
  tags: {required: false, shape: list, from: tag-vocabulary}
  sources: {required: true, shape: objects, keys: [name, url]}
  created: {required: true, shape: date}
```

```go
schema, err := validate.LoadSchema("schema.md")
if err != nil {
    return err
}
issues := schema.ValidateDocument("note.md", data)   // one document
issues, err = schema.ValidateDir("docs/")            // file, directory, or glob
```

Each issue carries `File`, `Field`, and `Reason`. Generic shapes cover
string, single, list, objects, object, case, date, number, and
boolean. `from` references resolve at load time to the controlled
vocabulary files next to the schema: a bare name is looked up in the
schema directory, then in the parent directory's `assets/` and
`references/` folders and the parent itself; a name with a slash
resolves only below the schema directory. A reference that is absolute,
climbs out with `..`, or reaches outside through a symlink fails with
`validate.ErrVocabularyEscape`. Domain-specific shapes register
through `ShapeFunc` and receive the schema's `Ctx` value, so domain
semantics stay caller-side:

```go
schema.Ctx = myIndex
schema.RegisterShape("game", func(ctx any, field validate.Field, value any) []validate.Issue {
    // custom validation over ctx
    return nil
})
```

### Inline Style Rules

A field declaring `inline: true` pins the YAML style of its value.
`ValidateDocument` sees only the decoded data, where style is lost, so
the style checks run through `ValidateMetadata` on the parsed
metadata:

| Shape | `inline: true` means |
|-------|---------------------|
| `list`, `objects` | the sequence must be a flow list: `tags: [a, b]` |
| `object`, `case` | the mapping must be a flow map: `meta: {id: 1}` |
| `string` | no block scalars: `\|` and `>` are rejected |

```go
meta, _, err := markly.ParseFrontmatter(text)
if err != nil {
    return err
}
issues := schema.ValidateMetadata("note.md", meta)
```

The style checks apply to YAML frontmatter only; TOML has no block
style and always satisfies the rule.

## Heading Validation

The `validate` sub-package also checks a parsed heading list against a
caller-supplied expected list:

```go
md := markly.NewMDFileFromString(text, markly.WithFenceAwareHeadings())
content, _ := md.GetContent()
required := true
result, err := validate.ValidateHeadings(content.Headings,
    "identifier", "strict", "strict",
    []validate.ExpectedHeading{
        {Text: "supported_platforms", Level: 2, Required: &required},
        {Text: "Sources", Level: 3},   // nil required reads as required
    })
```

Each expected heading carries its level, a `required` flag (nil reads
as required), an optional per-item `text_mode`/`hierarchy_mode`
override, and `when_present` children enforced only when the parent is
found. Modes apply to the whole call and stay overridable per item:

| Mode | Values | Meaning |
|------|--------|---------|
| `text_mode` | `strict`, `permissive`, `identifier` | strict: case-folded, whitespace-collapsed equality. permissive: every expected word occurs as a whole word, order and extras free. identifier: case-fold plus separator folding; `_`, `-`, `/`, `&`, `.`, and similar punctuation become single spaces and the connectives `and`/`or` drop from both sides, so `supported_platforms` matches `Supported Platforms` and `privacy_&_security` matches `Privacy and Security`; word order still counts, and different or extra words still fail |
| `hierarchy_mode` | `strict`, `loose`, `ignore` | heading level equals the expected level, is the same depth or deeper, or is not compared |
| `order_mode` | `strict`, `loose` | expected headings in order with no extras, or other headings allowed between them |

Worked trace, the expected text `supported_platforms` against the
heading `## Supported Platforms`:

| text_mode | What it computes | Result |
|-----------|------------------|--------|
| `strict` | `"supported_platforms"` vs `"Supported Platforms"` | fail: `_` was never a space |
| `permissive` | `words("supported_platforms")` = `[supported_platforms]` | fail: the word equals neither `supported` nor `platforms` as a whole word |
| `identifier` | `identifierWords("supported_platforms")` = `supported platforms`; `identifierWords("Supported Platforms")` = `Supported Platforms`; case-folded equality | match |

The separator fold turns each punctuation run into one space, so
`public-reception`, `public.reception`, and `public reception` all
match the heading `Public Reception`, and dropping `and`/`or` from both
sides makes `privacy_&_security` match `Privacy and Security`. `&`
folds to a space first, which is why the ampersand pair composes with
the connective rule. Word order still counts: `Platforms Supported`
fails. Different words still fail: `Supported Operating Systems` is not
`supported_platforms`.

Issues carry `Field` (the heading text), `Reason`, and `Severity`
(`error` fails the check, `warning` does not). The mode strings are
plain Go constants; an unknown value returns an error from
`ValidateHeadings` before any comparison runs.

## License

MIT License - See [LICENSE](LICENSE) file for details.
