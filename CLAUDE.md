# Wasteland Development Conventions

## Project

Wasteland is a standalone CLI (`wl`) for the Wasteland federation protocol.
It was extracted from the gastown monorepo to evolve independently.

## Build & Test

- `make build` — compile `wl` binary to `bin/wl`
- `make check` — fmt-check, lint, vet, test (pre-commit runs this)
- `make test` — unit tests only
- `go test ./...` — run all tests

## Architecture

- `cmd/wl/` — CLI entry point and command handlers
- `internal/federation/` — core wasteland protocol (join, config, DoltHub API)
- `internal/commons/` — wl-commons database operations (wanted board CRUD)
- `internal/xdg/` — XDG base directory support for config/data paths
- `internal/style/` — terminal styling with lipgloss (Ayu theme)

## Conventions

- XDG paths: config in `~/.config/wasteland/`, data in `~/.local/share/wasteland/`
- Identity: `--display-name` and `--email` flags, falling back to `git config`
- No gastown imports — fully standalone
- Test doubles: hand-written fakes, no mock libraries
- CLI pattern: `runFoo()` wires deps, testable logic in separate functions
