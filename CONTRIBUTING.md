# Contributing to Wasteland

Thanks for your interest in contributing! Wasteland is experimental software, and we welcome contributions that help explore these ideas.

## Getting Started

1. Fork the repository
2. Clone your fork
3. Install prerequisites (see README.md)
4. Set up tooling and git hooks: `make setup`
5. Build and test: `make build && make check`

## Development Workflow

We use a direct-to-main workflow for trusted contributors. For external contributors:

1. Create a feature branch from `main`
2. Make your changes
3. Ensure quality gates pass: `make check`
4. Submit a pull request

### PR Branch Naming

**Never create PRs from your fork's `main` branch.** Always create a dedicated branch for each PR:

```bash
# Good - dedicated branch per PR
git checkout -b fix/session-startup upstream/main
git checkout -b feat/formula-parser upstream/main

# Bad - PR from main accumulates unrelated commits
git checkout main  # Don't PR from here!
```

Branch naming conventions:
- `fix/*` - Bug fixes
- `feat/*` - New features
- `refactor/*` - Code restructuring
- `docs/*` - Documentation only

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep functions focused and small
- Add comments for non-obvious logic
- Include tests for new functionality

## What to Contribute

Good first contributions:
- Bug fixes with clear reproduction steps
- Documentation improvements
- Test coverage for untested code paths
- Small, focused features

For larger changes, please open an issue first to discuss the approach.

## Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Keep the first line under 72 characters
- Reference issues when applicable

## Make Commands

Run `make help` to see all targets. Key commands:

| Command | What it does |
|---|---|
| `make setup` | Install tools (golangci-lint) and git hooks |
| `make build` | Compile `wl` binary with version metadata |
| `make install` | Build and install `wl` to `~/.local/bin` |
| `make check` | Fast quality gates: format check, lint, vet, unit tests |
| `make check-all` | All quality gates including integration tests |
| `make test` | Unit tests only |
| `make test-integration` | All tests including integration |
| `make lint` | Run golangci-lint |
| `make fmt` | Auto-fix formatting |
| `make fmt-check` | Fail if formatting would change files |
| `make vet` | Run `go vet` |
| `make cover` | Run tests with coverage report |
| `make clean` | Remove build artifacts |

Before submitting a PR, run:

```bash
make check
```

The pre-commit hook (installed by `make setup`) runs `make check` automatically on every commit.

## Questions?

Open an issue for questions about contributing. We're happy to help!
