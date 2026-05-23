# Contributing to pi-stream

## Prerequisites

- Go 1.23+
- `pi` binary on `$PATH` (for manual testing)
- golangci-lint (optional, for local linting) — install via:
  ```sh
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
  ```

## Development

All common tasks are defined in the `Makefile`. Run `make help` to see available targets:

| Command | Description |
|---------|-------------|
| `make test` | Run tests with `-race` |
| `make lint` | Run golangci-lint |
| `make fmt`  | Format code with `gofmt` |
| `make vet`  | Run `go vet` |
| `make tidy` | Run `go mod tidy` and verify no changes |
| `make check` | Run all checks: fmt, vet, staticcheck, lint, test, build |

Before submitting a PR, always run `make check` locally.

## Pull Requests

1. Create a feature branch from `main` using a descriptive name (e.g. `feat/add-tool-support`, `fix/exit-code-handling`)
2. Make focused, atomic commits with clear messages
3. Run `make check` — all checks must pass
4. Open a PR against `main` using the provided template
5. CI runs automatically on every PR — it executes linting, formatting, vet, staticcheck, `go mod tidy` verification, tests (Linux + macOS), and a build check
6. All CI checks must pass before merging
7. Maintainers will review and merge once approved
