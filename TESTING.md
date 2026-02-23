# Wasteland Testing Philosophy

## Three tiers, clear boundaries

### 1. Unit tests (`*_test.go` next to the code)

Test what the CODE does. Internal behavior, edge cases, precise failure
injection. These are fast and run everywhere.

- Use `t.TempDir()` for filesystem tests
- Use `require` for preconditions (fail immediately), `assert` for checks
- Construct exact broken states in Go — corrupt files, concurrent writes,
  duplicate IDs, missing directories
- No env vars for controlling behavior — pass dependencies directly
- Same package as the code under test (access to unexported functions)

When to use: corrupted data, concurrent writes, specific error types,
double-claim conflicts, rollback behavior, boundary conditions.

### 1b. Testscript CLI tests (`cmd/wl/testdata/*.txtar`)

Test what the **CLI shows the user**. Exact stdout/stderr output, exit codes,
flag validation, error messages. These are fast (in-process, no binary build)
and run everywhere alongside unit tests.

- Uses [go-internal/testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript)
- Each `.txtar` file runs in an isolated `$WORK` directory
- XDG dirs are isolated per script (no cross-contamination)
- Pre-seed config files via archive sections in `.txtar` files
- `exec wl <args>` runs the CLI in-process via `testscript.Main`

```
go test -v -run TestScripts ./cmd/wl/
```

When to use: verifying user-facing output, error messages, flag validation,
help text, subcommand discovery, config-dependent behavior (list, leave, join
when already joined).

### 2a. Offline integration tests (`test/integration/offline/`)

Test the wl binary end-to-end against real dolt databases using `file://`
remotes. No network access required. Runs in CI on every PR.

Each test gets its own isolated `testEnv` with temp XDG dirs — zero
cross-contamination. The `wl` binary is built once in `TestMain`.

- **Lifecycle tests**: post -> claim -> done full cycle, error cases
- **Sync tests**: file:// upstream remotes, `wl sync` and `--dry-run`

Run with: `make test-integration-offline`

When to use: verifying CLI behavior end-to-end, testing dolt interactions
without network, validating the full post/claim/done lifecycle.

### 2b. DoltHub integration tests (`test/integration/`)

Test that real pieces fit together with DoltHub. Need real dolt CLI,
real DoltHub, real filesystem. Runs only on push to main.

```go
//go:build integration

func TestRealDoltClone(t *testing.T) {
    // actually clones from DoltHub
}
```

When to use: proving the fakes are honest, smoke testing the real infra,
testing dolt CLI interactions with real DoltHub databases.

Run with: `go test -tags integration ./test/integration/`

## Decision guide

| Question you're testing | Tier |
|---|---|
| Does `wl browse` build correct SQL queries? | Unit test |
| Does `wl claim` fail for non-open items? | Unit test |
| Does CSV parsing handle quoted fields? | Unit test |
| Does SQL escaping prevent injection? | Unit test |
| Does the federation join workflow call steps in order? | Unit test |
| Does `wl post --type invalid` show the right error? | Testscript |
| Does `wl list` show "No wastelands joined" when empty? | Testscript |
| Does `wl leave` auto-resolve with one wasteland? | Testscript |
| Does `wl version` print the version string? | Testscript |
| Does `wl post` create a valid database row? | Offline integration |
| Does `wl claim` on an already-claimed item fail? | Offline integration |
| Does `wl sync` pull from an upstream file:// remote? | Offline integration |
| Does a real dolt clone succeed from DoltHub? | DoltHub integration |
| Does `hop/wl-commons` schema match expected tables/columns? | DoltHub integration |
| Are all `wanted` statuses/priorities/types valid? | DoltHub integration |

## Test doubles

No mock libraries. No `gomock`. No `mockgen`. Every test double is a
hand-written concrete type that lives in the same package as the
interface it implements.

### The do*() function pattern

Every CLI command splits into two functions:

- **`runFoo()`** — wires up real dependencies (loads config, creates store),
  then calls `doFoo()` or the testable business logic function.
- **`doFoo()` / business logic** — pure logic. Accepts all dependencies as arguments.

Unit tests call the business logic directly with fakes:
```go
store := newFakeWLCommonsStore()
item, err := claimWanted(store, "w-abc123", "my-rig")
```

### Error injection

Per-field error injection on fakes:
```go
store := newFakeWLCommonsStore()
store.EnsureDBErr = fmt.Errorf("server down")
```
