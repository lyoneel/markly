# Changelog

All notable changes in the markly project are documented in this file. The format is based on Keep a Changelog, and the project follows Semantic Versioning.

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
