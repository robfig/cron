# Repository Guidelines

## Project Structure & Module Organization
- Root-level Go package for cron scheduling logic (e.g., `cron.go`, `parser.go`, `spec.go`, `chain.go`, `logger.go`, `option.go`).
- Tests live alongside sources as `*_test.go` files (e.g., `cron_test.go`, `parser_test.go`).
- Documentation is in `README.md` and package docs in `doc.go`.
- CI config lives in `.github/workflows/go.yml` (GitHub Actions) and `.travis.yml`.
- Module path is `github.com/robfig/cron/v3` in `go.mod`.

## Build, Test, and Development Commands
- `go build -v ./...` — compile all packages (matches CI).
- `go test -v ./...` — run the full test suite with verbose output.
- `go test ./...` — faster local run when you don’t need verbose logs.

## Coding Style & Naming Conventions
- Run `gofmt` on all Go files before committing; Go uses tabs for indentation.
- Exported identifiers use `CamelCase`; unexported use `camelCase` (follow Go acronyms like `URL`, `ID`).
- Keep files and types aligned with existing naming patterns (e.g., `spec.go` + `spec_test.go`).

## Testing Guidelines
- Use the standard Go `testing` package.
- Test functions should be named `TestXxx`; place tests in `*_test.go` next to the code they cover.
- Prefer table-driven tests for parsers/schedulers; add regression tests for bug fixes.

## Commit & Pull Request Guidelines
- Commit messages are short and imperative; common prefixes include `fix:`, `docs:`, `CI:`, and `README:`.
- PRs should include a concise description, rationale, and test output (e.g., `go test -v ./...`).
- Update `README.md` when public API behavior changes.

## Compatibility & Configuration Notes
- The module is v3; imports must use `github.com/robfig/cron/v3`.
- `go.mod` targets Go 1.12, while CI uses Go 1.20—avoid features newer than 1.12 unless explicitly agreed.
