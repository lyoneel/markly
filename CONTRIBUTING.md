# Contributing to markly

Thank you for considering a contribution. This document explains how to set up the development environment, follow the project conventions, and submit changes.

> Main repository: https://gitlab.com/lyoneel/markly
> Any other host that serves this repository is a mirror. Open issues
> and merge requests on GitLab.

## Code of Conduct

This project keeps collaboration practical and respectful. Treat maintainers and contributors as professional peers.

## Development Environment Setup

1. Install Go 1.26 or later. Check with `go version`.
2. Clone the repository. The module depends on dirly, its sibling library:

```bash
git clone https://gitlab.com/lyoneel/markly.git
cd markly
```

3. Resolve the dependency. After the public release of dirly, `go mod tidy` fetches it from the module proxy. For local development against a sibling checkout, add a replace directive:

```bash
go mod edit -replace gitlab.com/lyoneel/dirly=../dirly
```

4. Confirm the toolchain works:

```bash
go build ./...
go test ./...
```

## Project Layout

| Path | Content |
|------|---------|
| `mdfile.go` | The `MDFile` type: constructors, lazy and eager loading, save paths |
| `mdmetadata.go` | The `MDMetadata` typed accessor |
| `mdcontent.go`, `mdheadings.go` | Content and heading containers with line ranges |
| `mdbody.go`, `mdcheckbox.go`, `mdsections.go` | Body, checkbox, and section editing |
| `mdfolder.go` | The `MDFolder` loader and dependency graph |
| `*_test.go` | Parsing, metadata, folder, and boundary tests |

## Code Style Guidelines

1. Follow the conventions of Effective Go and the Go Code Review Comments guide.
2. Run `gofmt` on every changed file. The project has no custom formatter configuration.
3. Keep the public API stable within a major version. Breaking changes require a major version bump and a CHANGELOG entry.
4. Document every exported symbol with a doc comment that starts with the symbol name.
5. Keep the lazy-loading contract: constructors must not load content, and accessors must return zero values instead of errors when data is absent.
6. Preserve line-number accuracy. Every parser change needs a test that asserts the recorded line ranges.

## Testing Requirements

1. Run the full suite with race detection before you submit anything:

```bash
go test -race -cover ./...
```

2. New features need table-driven tests in the style of the existing `mdfile_*_test.go` and `mdfolder_test.go` files.
3. Bug fixes need a regression test that fails without the fix.
4. Keep coverage at or above the current 89 percent. The CI pipeline runs vet, tests with coverage, and a build on every merge request.
5. Tests must pass with `-race`. The folder loader and lazy loading paths must stay safe for concurrent use.

## Git Workflow

1. The default branch is the integration target. Create a feature branch from it:

```bash
git checkout -b feat/my-change
```

2. Use conventional commit messages in the form `type: message` or `type!: message` for breaking changes. Known types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`, `perf`.

```bash
git commit -m "feat: add section title helper for rewrites"
```

3. Keep each commit focused. One logical change per commit.
4. Rebase your branch on the default branch before you open a merge request.

## Pull Request Process

1. Push your branch to the same repository and open a merge request on GitLab.
2. Fill in the merge request template: describe the change, the type, and how you tested it.
3. The CI pipeline must pass: vet, tests with coverage, build.
4. Update documentation in the same change set:
   - README.md for user-facing behavior
   - DEVELOPMENT.md for internals and extension points
   - CHANGELOG.md under an Unreleased heading

## Code Review Expectations

1. A maintainer reviews every merge request. Expect a review within a few days.
2. Reviewers check correctness, test coverage, API stability, and documentation accuracy.
3. Address review comments with new commits; keep the merge request history readable.
4. Squash only at the discretion of the maintainer who merges.

## Onboarding for New Contributors

1. Read the README overview, then DEVELOPMENT.md for the type documentation and graph internals.
2. Pick an issue labeled with the bug label as a first change.
3. Build and run the suite first; the tests document the expected parsing behavior for every frontmatter format.
4. Ask questions in the merge request or in an issue. Questions in the open are welcome.

## License

The project uses the MIT license. All contributions are submitted under the MIT license. See the LICENSE file for the full text.
