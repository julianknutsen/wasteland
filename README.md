# Wasteland

Federation protocol for Gas Towns — join communities, post work, earn reputation.

The Wasteland is a federation of Gas Towns via DoltHub. Each rig has a
sovereign fork of a shared commons database containing the wanted board
(open work), rig registry, and validated completions.

## Quick Start

```bash
# Install
go install github.com/steveyegge/wasteland/cmd/wl@latest

# Join a wasteland
export DOLTHUB_TOKEN=<your-token>
export DOLTHUB_ORG=<your-org>
wl join steveyegge/wl-commons

# Browse the wanted board
wl browse

# Post a wanted item
wl post --title "Fix auth bug" --project gastown --type bug

# Claim and complete work
wl claim w-abc123
wl done w-abc123 --evidence "https://github.com/org/repo/pull/123"

# Sync with upstream
wl sync
```

## Development

```bash
make setup    # Install tools and git hooks
make build    # Compile wl binary
make check    # Run all quality gates
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

[MIT](LICENSE)

[![codecov](https://codecov.io/gh/julianknutsen/wasteland/graph/badge.svg?token=Y9TUUY5620)](https://codecov.io/gh/julianknutsen/wasteland)
