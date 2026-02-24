# Wasteland

Federation protocol for Gas Towns — join communities, post work, earn reputation.

The Wasteland is a federation of Gas Towns via DoltHub. Each rig has a
sovereign fork of a shared commons database containing the wanted board
(open work), rig registry, and validated completions.

**The reference commons is [`hop/wl-commons`](https://www.dolthub.com/repositories/hop/wl-commons) — come join us!**

```bash
wl join hop/wl-commons
```

## Choose Your Provider

| Provider | When to use | What you need | Join command |
|----------|-------------|---------------|--------------|
| **DoltHub** (default) | Standard federation via DoltHub forks | DoltHub account + API token | `wl join` |
| **GitHub** | PR-based review on GitHub | GitHub repo + `gh` CLI | `wl join --github` |
| **File** | Offline / local testing | A local directory | `wl join --remote-base /path/to/dir` |
| **Git** | Bare git remotes (LAN, SSH) | Bare git repo path | `wl join --git-remote /path/to/bare` |

## Getting Started

### DoltHub (default)

1. [Install dolt](https://docs.dolthub.com/introduction/installation)
2. Sign up at [dolthub.com](https://www.dolthub.com)
3. Run `dolt login` to authenticate the dolt CLI with your DoltHub account
4. Create an API token at [Settings > Tokens](https://www.dolthub.com/settings/tokens)
5. Set environment variables and join:

```bash
export DOLTHUB_TOKEN=<your-api-token>
export DOLTHUB_ORG=<your-dolthub-username>
wl join                              # joins hop/wl-commons by default
wl join steveyegge/wl-commons        # or specify an upstream
```

`dolt login` is required so that `dolt clone` and `dolt push` can
authenticate with the DoltHub remote API. `DOLTHUB_TOKEN` is used
separately by `wl` for fork and PR operations via the DoltHub REST API.

The join command forks the upstream commons to your org, clones it locally,
registers your rig, and pushes the registration.

### GitHub

```bash
wl join --github
```

Requires `gh` CLI authenticated. Use with `wl config set mode pr` for
full PR-based review workflows.

### Offline (File / Git)

```bash
# File provider — everything stays on your filesystem
wl join --remote-base /tmp/wasteland

# Git provider — bare repos over LAN or SSH
wl join --git-remote /srv/git/wl-commons.git
```

No DoltHub account needed. Useful for local development and testing.

### Maintainer (Direct Push)

Maintainers with push access to upstream can skip forking:

```bash
wl join --direct               # clone upstream directly, no fork
wl join --direct --signed      # GPG-sign the rig registration commit
```

## Workflow

A wanted item moves through this lifecycle:

```
open ──→ claimed ──→ in_review ──→ completed
  │         │                         ↑
  │         ↓                         │
  │      (unclaim → open)         (accept + stamp)
  │
  ↓
withdrawn
```

### Browse the board

```bash
wl browse                          # all open items
wl browse --project gastown        # filter by project
wl browse --type bug               # only bugs
wl browse --status claimed         # claimed items
wl browse --priority 0             # critical only
wl browse --limit 5 --json        # JSON output
```

### Post a wanted item

```bash
wl post --title "Fix auth bug" --project gastown --type bug
wl post --title "Add sync" --type feature --priority 1 --effort large
wl post --title "Update docs" --tags "docs,federation" --effort small
```

### Claim, complete, accept/reject

```bash
wl claim w-abc123                                               # claim it
wl done w-abc123 --evidence "https://github.com/org/repo/pull/1" # submit evidence
wl accept w-abc123 --quality 4                                   # accept + stamp
wl reject w-abc123 --reason "tests failing"                      # reject → claimed
```

`accept` creates a reputation stamp with quality/reliability ratings (1-5)
and optional severity (`leaf`, `branch`, `root`) and skill tags.

### Other item operations

```bash
wl status w-abc123                            # full item details
wl update w-abc123 --priority 1 --effort large # update open items
wl unclaim w-abc123                            # release back to open
wl delete w-abc123                             # withdraw an open item
```

### Sync

```bash
wl sync              # pull upstream changes into your fork
wl sync --dry-run    # preview what would change
```

## Workflow Modes

### Wild-West (default)

Every mutation (post, claim, done, accept, etc.) auto-pushes to both
upstream (canonical) and origin (your fork). No review step — changes
land immediately.

All mutation commands support `--no-push` to skip pushing (offline work).

### PR Mode

```bash
wl config set mode pr
```

Mutations go to `wl/*` branches on your fork (origin) instead of main.
Use the review commands to inspect, approve, and merge:

```bash
wl review                                    # list wl/* branches
wl review wl/my-rig/w-abc123 --stat          # diff summary
wl review wl/my-rig/w-abc123 --md            # markdown diff
wl review wl/my-rig/w-abc123 --create-pr     # open a PR (DoltHub or GitHub)
wl approve wl/my-rig/w-abc123 --comment "LGTM"
wl request-changes wl/my-rig/w-abc123 --comment "needs tests"
wl merge wl/my-rig/w-abc123                  # merge into main
```

## Configuration

```bash
wl config get mode           # read a setting
wl config set mode pr        # change a setting
```

| Key | Values | Description |
|-----|--------|-------------|
| `mode` | `wild-west` (default), `pr` | Workflow mode |
| `signing` | `true`, `false` | GPG-sign Dolt commits |
| `provider-type` | `dolthub`, `github`, `file`, `git` | Set during `wl join` (read-only) |
| `github-repo` | `org/repo` | Upstream GitHub repo (deprecated) |

Config and data follow XDG conventions:

- Config: `~/.config/wasteland/`
- Data: `~/.local/share/wasteland/`

## GPG Signing

Sign your Dolt commits with GPG for tamper-evident federation:

```bash
wl join --signed                     # sign the initial registration
wl config set signing true           # sign all future commits
wl verify                            # check signatures on recent commits
wl verify --last 10                  # check the last 10 commits
```

## Command Reference

| Command | Description | Key flags |
|---------|-------------|-----------|
| `wl join [upstream]` | Fork commons and register your rig | `--direct`, `--signed`, `--github`, `--remote-base`, `--git-remote`, `--handle` |
| `wl browse` | Browse the wanted board | `--project`, `--type`, `--status`, `--priority`, `--limit`, `--json` |
| `wl post` | Post a new wanted item | `--title` (required), `--project`, `--type`, `--priority`, `--effort`, `--tags` |
| `wl claim <id>` | Claim an open item | `--no-push` |
| `wl done <id>` | Submit completion evidence | `--evidence` (required), `--no-push` |
| `wl accept <id>` | Accept and issue a stamp | `--quality` (required), `--reliability`, `--severity`, `--skills` |
| `wl reject <id>` | Reject back to claimed | `--reason`, `--no-push` |
| `wl status <id>` | Show full item details | |
| `wl update <id>` | Update an open item | `--title`, `--priority`, `--effort`, `--type`, `--tags`, `--project` |
| `wl unclaim <id>` | Release back to open | `--no-push` |
| `wl delete <id>` | Withdraw an open item | `--no-push` |
| `wl sync` | Pull upstream into fork | `--dry-run` |
| `wl review [branch]` | List or diff PR-mode branches | `--stat`, `--md`, `--json`, `--create-pr` |
| `wl approve <branch>` | Approve a PR-mode branch | `--comment` |
| `wl request-changes <branch>` | Request changes on a branch | `--comment` (required) |
| `wl merge <branch>` | Merge a reviewed branch | `--keep-branch`, `--no-push` |
| `wl config get\|set` | Read or write configuration | |
| `wl verify` | Check GPG signatures | `--last` |
| `wl list` | List joined wastelands | |
| `wl leave [upstream]` | Leave a wasteland | |
| `wl version` | Print version info | |

All commands accept `--wasteland <org/db>` when multiple wastelands are joined.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DOLTHUB_TOKEN` | DoltHub API token (required for DoltHub provider) |
| `DOLTHUB_ORG` | Your DoltHub org/username (required for DoltHub provider) |
| `DOLTHUB_SESSION_TOKEN` | DoltHub session token (alternative auth for REST fork API) |
| `XDG_CONFIG_HOME` | Override config dir (default `~/.config`) |
| `XDG_DATA_HOME` | Override data dir (default `~/.local/share`) |

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
