# Changelog

All notable changes in the markly project are documented in this file. The format is based on Keep a Changelog, and the project follows Semantic Versioning.

## v1.5.0 - 2026-09-22

### Added

- YAML style introspection on metadata: `IsFlowMapping(key)` and `IsBlockScalar(key)` join `IsFlowSequence`, and `ParseFrontmatter` now attaches the node tree so the style helpers work on its result
- The `validate` sub-package supports an `inline: true` field rule with `Schema.ValidateMetadata(name, meta)`: list and objects fields must be flow lists (`[a, b]`), object and case fields must be flow maps (`{a: 1}`), and string fields must not be block scalars (`|`, `>`). The style checks run on YAML frontmatter only; `ValidateDocument` on the plain data map stays style-blind

## v1.4.0 - 2026-09-22

### Added

- `ParseFrontmatter(text)`: one-shot frontmatter parse returning the metadata plus the body after the block; returns `ErrNoFrontmatter` when the delimiters are absent and a `FrontmatterError` (document line plus wrapped error) when the block fails to parse. `GetMetadata` keeps its current contract
- Coercing metadata accessors: `GetCoercedString` reads numeric scalars as their decimal text, `GetCoercedInt` reads decimal strings as ints; `GetString` and `GetInt` keep their semantics
- `WithStrictDuplicates()` option: a repeated top-level frontmatter key fails the parse with a `FrontmatterError` naming the key and the line of the repeat; without the option, behavior is unchanged
- `FindSectionBody(slug)`: the text of a `##` section up to the next heading of the same or higher level, with leading and trailing blank lines trimmed
- New `validate` sub-package: schema-driven frontmatter validation using the `fields`/`config` schema format. Generic shapes (string, single, list, objects, object, case, date, number, boolean), value vocabularies through `from` references resolved at load time, pluggable domain shapes through `ShapeFunc` registration, and typed `Issue{File, Field, Reason}` findings; `ValidateFile` and `ValidateDir` cover one document, a directory tree, or a glob

## v1.3.0 - 2026-09-22

### Added

- Package-level `SplitFrontmatter(text)` helper: split a markdown document into the raw frontmatter block (without delimiters) and the body after it; supports the YAML and TOML delimiter formats and reports whether frontmatter exists. No change to existing APIs

## v1.2.0 - 2026-08-17

### Changed

- Breaking: flatten the package to a single root package `markly` (the former `markly` subpackage is gone)
- Module renamed to `markly` and dependency switched to dirly
- Documentation now refers to the renamed dirly package
- Remove the leftover subdirectory after flattening

## v1.1.0 - 2026-08-14

### Added

- Attach metadata to documents without frontmatter
- In-memory construction and document manipulation: `NewMDFileFromString`, `NewMDFileFromBytes`, `Save`, `SaveAtomic`, body line editing, first-heading helpers, section helpers, checkbox rewrites
- Dot-directory skip and error collection for `MDFolder` via `WithSkipDotDirs` and `Errors()`
- Loading options: `WithRawScalars` and `WithFenceAwareHeadings`

### Fixed

- Documentation for document manipulation and folder options
- Test coverage for document manipulation and folder options

## v1.0.0 - 2026-08-10

### Added

- First stable release of the standalone library
- Automatic YAML and TOML frontmatter extraction with line number tracking and format detection
- Lazy content loading with explicit `LoadContent()`, plus eager constructors
- ATX-style heading detection with line ranges
- Typed metadata access via `MDMetadata`: string, int, bool, list, map, and raw accessors, struct unmarshaling
- Directory-based discovery with dependency graph resolution, cycle detection, and topological load order
- Metadata filtering and callback iteration
- Batch read and write operations
- Pluggable YAML frontmatter serializer

### Dependencies

- `gitlab.com/lyoneel/dirly` for file discovery
- `gopkg.in/yaml.v3` for YAML frontmatter
- `github.com/pelletier/go-toml/v2` for TOML frontmatter

## Statistics

- 15 commits across all tags
- 34 tracked files, 29 Go source files, about 6300 lines of Go
- Test coverage 89.2 percent of statements
