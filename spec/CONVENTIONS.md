# Conventions

## Language & Toolchain

- Go 1.22+
- Use `make` targets instead of invoking `go` commands directly; the `Makefile` wraps formatting, linting, code generation, build, and test with the correct flags and dependencies.
  - `make fmt` — format code (wraps `go fmt`)
  - `make vet` — vet code (wraps `go vet`)
  - `make lint` / `make lint-fix` — run `golangci-lint`
  - `make manifests generate` — regenerate CRDs, RBAC, DeepCopy code via `controller-gen`
  - `make build` — build the manager binary
  - `make test` — run unit tests
  - `make test-e2e` — run end-to-end tests against a Kind cluster
- `gofmt` for formatting (enforced via `make fmt`)
- `golangci-lint` for linting (enforced via `make lint`)

## Naming

| Kind | Style | Example |
|---|---|---|
| Package | lowercase, no underscore | `agentrunner` |
| File | snake_case | `agent_runner.go` |
| Exported | PascalCase | `RunAgent` |
| Unexported | camelCase | `runAgent` |
| Constants | PascalCase or ALL_CAPS only for universe-block consts | `MaxRetries` |
| Interface | noun or `-er` suffix | `Runner`, `MessageSender` |
| Test file | `<file>_test.go` | `agent_runner_test.go` |

## Code Location

Define and follow these placement rules:

- Production source code goes under `./`. e.g. `./main.go`, `./cmd/root.go`
- Unit tests live next to source files in the same package using `_test.go` suffix.
- End-to-end tests go under `./test/e2e`.
- Race condition tests go under `./test/race`.
- Shared test helpers and fixtures for non-unit tests should also go somewhere under `./test/`.

## Error Handling

- Never ignore errors
- Wrap with context: `fmt.Errorf("doing X: %w", err)`
- Define sentinel errors at package level: `var ErrNotFound = errors.New("not found")`
- Return errors, do not panic in library code

## Testing

- Use `testing` stdlib + `github.com/stretchr/testify`
- Test function: `TestFoo_scenario` (e.g. `TestRunAgent_returnsErrOnTimeout`)
- Table-driven tests preferred for multiple cases
- Mock interfaces, not concrete types

## Imports

Group in this order (separated by blank lines):

```go
import (
    "stdlib"

    "external/module"
    "internal/package"
)
```

## Code Style

- Prefer short variable names in small scopes (`i`, `err`, `ok`)
- Return early on error (guard clauses)
- No naked returns
- Context (`context.Context`) is always the first parameter when present
- Unexported struct fields by default; export only what is needed
- Interfaces belong in the package that *uses* them, not the one that implements them
