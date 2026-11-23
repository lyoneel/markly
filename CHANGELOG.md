# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `WithSymlinkPolicy(SymlinkPolicy)` loader option for `MDFolder`: `FollowWithinRoot` (default), `FollowAny`, or `NoFollow`, plus `MDFolder.SymlinkPolicy()`. `SymlinkPolicy`, its constants, `DefaultSymlinkPolicy`, `ErrPathEscapesRoot`, and `ErrSymlinkNotAllowed` are re-exported from dirly, and an unknown policy makes `NewMDFolder` fail with the new `ErrInvalidSymlinkPolicy`.

### Changed

- markly now builds on dirly v1.3.0, and `MDFolder` reads its files through dirly's `Directory.Open` instead of opening them directly, so the symlink policy applies to metadata parsing and to every lazy `LoadContent`. Files opened with `NewMDFile`, `NewMDFileWithContent`, or the in-memory constructors are unaffected.

### Fixed

- An `MDFolder` whose path is itself a symlink (for example `~/notes -> ~/Sync/notes`) now discovers its files; previously it found none.

### Security

- GO-2026-4602: **Breaking:** the minimum Go version is now 1.26.11 (also required by dirly v1.2.0 and later), which clears this and the other standard-library advisories reported against 1.26.0; under 1.26.0 the advisory was reachable through `validate.CollectDocs`. CI now pins the `golang:1.26.11` image. To upgrade, build with Go 1.26.11 or later.
- **Breaking:** An `.md` symlink inside an `MDFolder` that resolves outside the folder is no longer read; it is skipped and reported in `Errors()` with an error wrapping `ErrPathEscapesRoot`. Under the default policy, reads go through an `os.Root` at the resolved folder root, so a file swapped for an escaping symlink after discovery is refused as well. If your folders deliberately link to files elsewhere, use `NewMDFolder(path, markly.WithSymlinkPolicy(markly.FollowAny))`.

## [1.7.1] - 2026-10-10

### Added

- `text_mode: identifier` in the `validate` heading check (`ValidateHeadings` and the per-item `ExpectedHeading.TextMode`): case folding plus separator folding, where `_`, `-`, `/`, `&`, `.`, and similar punctuation become single spaces and the connectives `and`/`or` drop from both sides, so identifier-styled keys (`supported_platforms`, `public-reception`, `privacy_&_security`) match their human title spellings (`Supported Platforms`, `Public Reception`, `Privacy and Security`). Word order still counts and different or extra words still fail; the existing `strict` and `permissive` behavior is unchanged. Worked trace, expected text `supported_platforms` against heading `## Supported Platforms`: `strict` fails because "_" was never a space; `permissive` fails because the whole word `supported_platforms` equals neither `supported` nor `platforms`; `identifier` folds both sides to `supported platforms` / `Supported Platforms` and matches case-insensitively.

## [1.6.0] - 2026-09-24

### Added

- `ValidateHeadings` in the `validate` sub-package: compares a parsed heading list with a caller-supplied expected list and reports mismatches in text, hierarchy, order, and `when_present` children.

## [1.5.0] - 2026-09-22

### Added

- YAML style introspection on metadata: `IsFlowMapping(key)` and `IsBlockScalar(key)` join `IsFlowSequence`, and `ParseFrontmatter` now attaches the node tree so the style helpers work on its result.
- The `validate` sub-package supports an `inline: true` field rule with `Schema.ValidateMetadata(name, meta)`: list and objects fields must be flow lists (`[a, b]`), object and case fields must be flow maps (`{a: 1}`), and string fields must not be block scalars (`|`, `>`). The style checks run on YAML frontmatter only; `ValidateDocument` on the plain data map stays style-blind.

## [1.4.0] - 2026-09-22

### Added

- `ParseFrontmatter(text)`: one-shot frontmatter parse returning the metadata plus the body after the block; returns `ErrNoFrontmatter` when the delimiters are absent and a `FrontmatterError` (document line plus wrapped error) when the block fails to parse. `GetMetadata` keeps its current contract.
- Coercing metadata accessors: `GetCoercedString` reads numeric scalars as their decimal text, `GetCoercedInt` reads decimal strings as ints; `GetString` and `GetInt` keep their semantics.
- `WithStrictDuplicates()` option: a repeated top-level frontmatter key fails the parse with a `FrontmatterError` naming the key and the line of the repeat; without the option, behavior is unchanged.
- `FindSectionBody(slug)`: the text of a `##` section up to the next heading of the same or higher level, with leading and trailing blank lines trimmed.
- New `validate` sub-package: schema-driven frontmatter validation using the `fields`/`config` schema format. Generic shapes (string, single, list, objects, object, case, date, number, boolean), value vocabularies through `from` references resolved at load time, pluggable domain shapes through `ShapeFunc` registration, and typed `Issue{File, Field, Reason}` findings; `ValidateFile` and `ValidateDir` cover one document, a directory tree, or a glob.

## [1.3.0] - 2026-09-22

### Added

- Package-level `SplitFrontmatter(text)` helper: split a markdown document into the raw frontmatter block (without delimiters) and the body after it; supports the YAML and TOML delimiter formats and reports whether frontmatter exists. No change to existing APIs.

## [1.2.0] - 2026-08-17

### Changed

- **Breaking:** The package is flattened to a single root package `markly`, the module is renamed to `gitlab.com/lyoneel/markly`, and the leftover subdirectory is gone. To upgrade, import `gitlab.com/lyoneel/markly` instead of the former subpackage path.
- markly now depends on dirly (the renamed directory library), and the documentation refers to it by its new name.

## [1.1.0] - 2026-08-14

### Added

- Attach metadata to documents without frontmatter.
- In-memory construction and document manipulation: `NewMDFileFromString`, `NewMDFileFromBytes`, `Save`, `SaveAtomic`, body line editing, first-heading helpers, section helpers, checkbox rewrites.
- Dot-directory skip and error collection for `MDFolder` via `WithSkipDotDirs` and `Errors()`.
- Loading options: `WithRawScalars` and `WithFenceAwareHeadings`.

### Fixed

- Documentation for document manipulation and folder options.
- Test coverage for document manipulation and folder options.

## [1.0.0] - 2026-08-10

### Added

- First stable release of the standalone library, built on dirly for file discovery, `gopkg.in/yaml.v3` for YAML frontmatter, and `github.com/pelletier/go-toml/v2` for TOML frontmatter.
- Automatic YAML and TOML frontmatter extraction with line number tracking and format detection.
- Lazy content loading with explicit `LoadContent()`, plus eager constructors.
- ATX-style heading detection with line ranges.
- Typed metadata access via `MDMetadata`: string, int, bool, list, map, and raw accessors, struct unmarshaling.
- Directory-based discovery with dependency graph resolution, cycle detection, and topological load order.
- Metadata filtering and callback iteration.
- Batch read and write operations.
- Pluggable YAML frontmatter serializer.

[Unreleased]: https://gitlab.com/lyoneel/markly/-/compare/v1.7.1...HEAD
[1.7.1]: https://gitlab.com/lyoneel/markly/-/compare/v1.6.0...v1.7.1
[1.6.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.5.0...v1.6.0
[1.5.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.4.0...v1.5.0
[1.4.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.3.0...v1.4.0
[1.3.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.2.0...v1.3.0
[1.2.0]: https://gitlab.com/lyoneel/markly/-/tags/v1.2.0
