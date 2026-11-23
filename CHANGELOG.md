# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.8.1] - 2026-11-23

### Added

- `WithMaxFileSize(limit)` for an `MDFile` and `WithFolderMaxFileSize(limit)` for every file an `MDFolder` loads set the largest document, in bytes, read from disk; a limit of 0 or less disables the check. The default is the new `DefaultMaxFileSize` (64 MiB, four times `MaxLineLength`), and an oversized file fails with an error wrapping the new `ErrFileTooLarge`.

### Changed

- markly now uses `go.yaml.in/yaml/v3` v3.0.6, the maintained drop-in successor of the archived `gopkg.in/yaml.v3`, which is no longer a dependency, and `github.com/pelletier/go-toml/v2` v2.4.3 (from v2.2.4).
- `GetLoadOrder`, `DetectCycles`, and `GetAllWithDependencies` run in time linear in files plus dependencies, after one sort of the names, and walk iteratively, so chains of any depth resolve without recursion. `GetAllWithDependencies` on a 10,000-file chain takes about 3 ms and 3 MB instead of 4.6 s and 6 GB. The load order, the files returned, and the errors are unchanged.
- `validate.LoadSchema` wraps the error from reading the schema or a vocabulary file instead of flattening it to text, so `errors.Is` works on it (for example with `os.ErrNotExist`).
- CI runs `go mod verify`, a `gofmt` check, `go vet`, `staticcheck` 2026.2.2, `govulncheck` v1.10.0, `go test -race -cover`, and `go build` in the `golang:1.26.11` image pinned by digest.

### Fixed

- `DetectCycles` returns the same cycles in the same order on every run; it walked the files in map order before.

### Security

- **Breaking:** Files read from disk are limited to `DefaultMaxFileSize` (64 MiB) by default. A larger file, or one that grows past the limit while it is read, fails `GetMetadata` and `LoadContent` with an error wrapping `ErrFileTooLarge` instead of being read into memory whole, and an `MDFolder` skips it and reports it in `Errors()`. markly reads at most one byte past the limit. The `validate` package applies the same limit to the documents, schemas, and vocabulary files it reads. To upgrade, nothing is needed for documents under 64 MiB; to read larger ones, pass `WithMaxFileSize` or `WithFolderMaxFileSize` with a higher limit, or 0 to disable the check.

## [1.8.0] - 2026-11-23

### Added

- `WithSymlinkPolicy(SymlinkPolicy)` loader option for `MDFolder`: `FollowWithinRoot` (default), `FollowAny`, or `NoFollow`, plus `MDFolder.SymlinkPolicy()`. `SymlinkPolicy`, its constants, `DefaultSymlinkPolicy`, `ErrPathEscapesRoot`, and `ErrSymlinkNotAllowed` are re-exported from dirly, and an unknown policy makes `NewMDFolder` fail with the new `ErrInvalidSymlinkPolicy`.
- `MDFile.Render() (string, error)`: the serialized document, or an error wrapping the new `ErrMetadataSerialization` when the metadata cannot be serialized.
- Exported errors: `ErrExcessiveAliasing`, `ErrMetadataSerialization`, `ErrMultilineText`, `ErrInvalidCheckboxMarker`, `ErrNotADirectory`, and `validate.ErrVocabularyEscape`.
- Constants `MaxLineLength` (16 MiB, the longest line a document may contain) and `DefaultFilePerm` (0644, the mode of newly created files).

### Changed

- markly now builds on dirly v1.3.0, and `MDFolder` reads its files through dirly's `Directory.Open` instead of opening them directly, so the symlink policy applies to metadata parsing and to every lazy `LoadContent`. Files opened with `NewMDFile`, `NewMDFileWithContent`, or the in-memory constructors are unaffected.
- **Breaking:** Frontmatter must start on line 1 and close with the same delimiter it opened with, and `GetMetadata`, `LoadContent`, `SplitFrontmatter`, and `ParseFrontmatter` now share one parser. A document that starts with a blank line, never closes its block, or mixes `---` and `+++` has no frontmatter and its whole text is body; before, such documents could pick up a later `---` (for example a horizontal rule) as metadata and duplicate text on save. To upgrade, move frontmatter to the first line and close it with its opening delimiter.
- **Breaking:** `AppendLine`, `InsertLine`, `SetSectionHeading`, `SetSectionBullet`, `InsertLineUnderHeading`, `SetFirstHeading`, and `RewriteCheckboxTitle` return `ErrMultilineText` for text containing `\n` or `\r`, and `SetCheckboxMarker` returns `ErrInvalidCheckboxMarker` for a marker other than `' '`, `'x'`, `'X'`, or `'~'`. To upgrade, split multi-line text and insert one line per call, or use `SetBody`.
- **Breaking:** `NewMDFolder` returns an error for a folder that does not exist (`os.IsNotExist` works) or a path that is not a directory (wrapping `ErrNotADirectory`); before, it returned a folder that found no files. To upgrade, handle the error from `NewMDFolder`, or create the folder first.
- Errors from opening a document are `*fs.PathError` values wrapping the cause, so `os.IsNotExist(err)` and `errors.Is(err, fs.ErrNotExist)` work on the errors from `GetMetadata`, `LoadContent`, and `NewMDFileWithContent`. The messages keep their `metadata not found` and `open failed` prefixes.
- Every `GetAll` or `GetAllByGlob` call replaces the folder's files, dependency graph, and `Errors()` with the result of that load. A discovery failure is reported in `Errors()` under the empty key `""`, and a file deleted between discovery and parsing is skipped.
- `GetLoadOrder` orders files without a constraint between them lexically, so its result is the same on every run.
- `SaveAtomic` syncs the temp file and the directory, keeps the existing file's mode and (best effort) owner, and writes through a symlink to its target instead of replacing the link. Files loaded from an `MDFolder` save through dirly's atomic writer under the folder's symlink policy, and fail with `dirly.ErrOwnership` when the owner cannot be kept.
- `Serialize` returns `""` when the metadata cannot be serialized.

### Fixed

- An `MDFolder` whose path is itself a symlink (for example `~/notes -> ~/Sync/notes`) now discovers its files; previously it found none.
- `Save` and `SaveAtomic` return an error wrapping `ErrMetadataSerialization` and leave the file untouched when the metadata cannot be serialized; before, they wrote the document with empty frontmatter.
- `SaveAtomic` no longer resets the file mode to 0600 or replaces a symlink with a regular file.
- Documents with a line longer than 64 KiB load; the limit is now `MaxLineLength` (16 MiB), and a longer line returns an error wrapping `bufio.ErrTooLong`.
- A dependency listed twice in one file is counted once, so `GetLoadOrder` and `DetectCycles` no longer report a cycle for it.
- A dependency field written as one string (`prerequisites: "a.md, b.md"`) is split on the separator set with `WithDepSeparator` (default `,`), with entries trimmed and empty entries and repeats dropped; before, the whole string was read as one dependency and `WithDepSeparator` had no effect. YAML lists are unchanged.
- `validate.CollectDocs` matches glob patterns that start with `./`, such as `./docs/*.md`.
- After `Save` or `SaveAtomic` of edited metadata, `FromLine` stays 2, the value a fresh parse reports, instead of becoming 1.
- In documents without frontmatter, heading `FromLine` values are no longer off by one, `ToLine` spans the section, and leading blank lines survive a save.
- The README and DEVELOPMENT examples compile against the current API, and a test type-checks them; SECURITY.md lists 1.8.x as the supported version.

### Security

- GO-2026-4602: **Breaking:** the minimum Go version is now 1.26.11 (also required by dirly v1.2.0 and later), which clears this and the other standard-library advisories reported against 1.26.0; under 1.26.0 the advisory was reachable through `validate.CollectDocs`. CI now pins the `golang:1.26.11` image. To upgrade, build with Go 1.26.11 or later.
- **Breaking:** An `.md` symlink inside an `MDFolder` that resolves outside the folder is no longer read; it is skipped and reported in `Errors()` with an error wrapping `ErrPathEscapesRoot`. Under the default policy, reads go through an `os.Root` at the resolved folder root, so a file swapped for an escaping symlink after discovery is refused as well. If your folders deliberately link to files elsewhere, use `NewMDFolder(path, markly.WithSymlinkPolicy(markly.FollowAny))`.
- **Breaking:** YAML alias expansion under `WithRawScalars` is budgeted at 10,000 values plus ten per node in the document, so an alias bomb of a few hundred bytes fails with `ErrExcessiveAliasing` instead of consuming gigabytes of memory. To upgrade, nothing is needed for ordinary documents; a document that legitimately expands aliases past the budget must be restructured.
- **Breaking:** `validate.LoadSchema` refuses a `from` reference that is absolute, climbs out with `..`, or resolves outside the schema location through a symlink, returning an error wrapping `ErrSchema` and `ErrVocabularyEscape`. A bare name is looked up in the schema directory, then in the parent directory's `assets/` and `references/` folders and the parent itself; a name with a slash resolves only below the schema directory. To upgrade, keep vocabulary files in those directories and reference them by relative name.

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

[Unreleased]: https://gitlab.com/lyoneel/markly/-/compare/v1.8.1...HEAD
[1.8.1]: https://gitlab.com/lyoneel/markly/-/compare/v1.8.0...v1.8.1
[1.8.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.7.1...v1.8.0
[1.7.1]: https://gitlab.com/lyoneel/markly/-/compare/v1.6.0...v1.7.1
[1.6.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.5.0...v1.6.0
[1.5.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.4.0...v1.5.0
[1.4.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.3.0...v1.4.0
[1.3.0]: https://gitlab.com/lyoneel/markly/-/compare/v1.2.0...v1.3.0
[1.2.0]: https://gitlab.com/lyoneel/markly/-/tags/v1.2.0
