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

### 2. Integration tests (`//go:build integration`)

Test that real pieces fit together. Need real dolt CLI, real DoltHub,
real filesystem. Run separately — not in CI by default.

```go
//go:build integration

func TestRealDoltClone(t *testing.T) {
    // actually clones from DoltHub
}
```

When to use: proving the fakes are honest, smoke testing the real infra,
testing dolt CLI interactions with real databases.

Run with: `go test -tags integration ./test/`

## Decision guide

| Question you're testing | Tier |
|---|---|
| Does `wl browse` build correct SQL queries? | Unit test |
| Does `wl claim` fail for non-open items? | Unit test |
| Does CSV parsing handle quoted fields? | Unit test |
| Does SQL escaping prevent injection? | Unit test |
| Does the federation join workflow call steps in order? | Unit test |
| Does a real dolt clone succeed from DoltHub? | Integration |

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
