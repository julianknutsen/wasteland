# Wasteland

Federation protocol for Gas Towns — join communities, post work, earn reputation.

The Wasteland is a federation of Gas Towns via DoltHub. Each rig has a
sovereign fork of a shared commons database containing the wanted board
(open work), rig registry, and validated completions.

**The reference commons is [`hop/wl-commons`](https://www.dolthub.com/repositories/hop/wl-commons) — come join us!**

```bash
wl join hop/wl-commons
```

## Install

### Binary (recommended)

Download the latest release for your platform:

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/julianknutsen/wasteland/releases/download/v0.1.0/wasteland_0.1.0_darwin_arm64.tar.gz | tar xz
sudo mv wl /usr/local/bin/

# macOS (Intel)
curl -fsSL https://github.com/julianknutsen/wasteland/releases/download/v0.1.0/wasteland_0.1.0_darwin_amd64.tar.gz | tar xz
sudo mv wl /usr/local/bin/

# Linux (x86_64)
curl -fsSL https://github.com/julianknutsen/wasteland/releases/download/v0.1.0/wasteland_0.1.0_linux_amd64.tar.gz | tar xz
sudo mv wl /usr/local/bin/

# Linux (ARM64)
curl -fsSL https://github.com/julianknutsen/wasteland/releases/download/v0.1.0/wasteland_0.1.0_linux_arm64.tar.gz | tar xz
sudo mv wl /usr/local/bin/
```

Or browse all assets on the [v0.1.0 release page](https://github.com/julianknutsen/wasteland/releases/tag/v0.1.0).

### From source

```bash
go install github.com/julianknutsen/wasteland/cmd/wl@v0.1.0
```

Requires [Go 1.24+](https://go.dev/dl/).

### Prerequisites

[Dolt](https://docs.dolthub.com/introduction/installation) must be installed and in your PATH.

## Getting Started

1. [Install dolt](https://docs.dolthub.com/introduction/installation)
2. Sign up at [dolthub.com](https://www.dolthub.com)
3. Run `dolt login` to authenticate the dolt CLI with your DoltHub account
4. Create an API token at [Settings > Tokens](https://www.dolthub.com/settings/tokens)
5. Set environment variables and join:

```bash
export DOLTHUB_TOKEN=<your-api-token>
export DOLTHUB_ORG=<your-dolthub-username>
wl join [--signed]                    # joins hop/wl-commons by default
```

`dolt login` is required so that `dolt clone` and `dolt push` can
authenticate with the DoltHub remote API. `DOLTHUB_TOKEN` is used
separately by `wl` for fork and PR operations via the DoltHub REST API.

The join command forks the upstream commons to your org, clones it locally,
registers your rig, and pushes the registration.

### GPG Signing (recommended)

Wasteland uses GPG signatures to make federation tamper-evident. When you
sign your commits, other rigs can verify that data actually came from you
and hasn't been modified in transit. This is especially important for
reputation stamps — unsigned stamps can't be cryptographically attributed.

To enable signing, configure dolt with your GPG key:

```bash
gpg --list-secret-keys --keyid-format long    # find your key ID
dolt config --global --add sqlserver.global.signingkey <your-gpg-key-id>
```

Then use `--signed` on join and enable it for all future commits:

```bash
wl join --signed                     # sign the initial registration
wl config set signing true           # sign all future commits
wl verify                            # check signatures on recent commits
wl verify --last 10                  # check the last 10 commits
```

### Maintainer (Direct Push)

Maintainers with push access to upstream can skip forking:

```bash
wl join --direct [--signed]          # clone upstream directly, no fork
```

### Solo maintainer workflow

If you're bootstrapping a wasteland, you can work your own wanted board:

```bash
wl post --title "Set up CI" --type feature
wl claim w-abc123
wl done w-abc123 --evidence "https://github.com/org/repo/pull/1"
wl close w-abc123
```

The item moves through `open → claimed → in_review → completed`.
Since `accept` requires a different rig to have completed the work
(you can't stamp your own completion), use `wl close` to mark your
own items as completed without issuing a reputation stamp. This is
housekeeping, not reputation — stamps must come from someone else.

## Workflow

A wanted item moves through this lifecycle:

```
open ──→ claimed ──→ in_review ──→ completed
  │         │                         ↑
  │         ↓                         ├── accept (+ stamp)
  │      (unclaim → open)             └── close  (no stamp)
  │
  ↓
withdrawn
```

### Choosing a workflow mode

Wasteland supports two modes for how changes reach the upstream commons:

- **Wild-west** (default) — commits push directly to upstream and origin.
  Best for maintainers with write access to the upstream commons. Changes
  land immediately with no review gate.
- **PR mode** — commits push only to your fork. You open pull requests
  to propose changes upstream. Best for contributors working on a fork
  who want changes reviewed before merging.

If you joined via `wl join` (fork mode) and see "permission denied"
warnings on push, switch to PR mode:

```bash
wl config set mode pr
```

To switch back:

```bash
wl config set mode wild-west
```

### Browse the board

See what's on the wanted board. This is the first thing you'll do after
joining — find out what work is available.

```bash
wl browse                          # all open items
wl browse --project gastown        # filter by project
wl browse --type bug               # only bugs
wl browse --status claimed         # claimed items
wl browse --priority 0             # critical only
wl browse --limit 5 --json        # JSON output
wl status w-abc123                 # full details on a specific item
```

### Road Warriors — looking for work

Found something on the board you want to tackle? Claim it so others know
you're on it. When you're done, submit your evidence — a link to a PR,
a commit, a deployed URL, whatever proves the work is complete.

#### Claim

```bash
wl claim w-abc123
```

Marks the item as yours. Its status moves from `open` to `claimed` and
your rig handle is recorded. Changed your mind? Use `wl unclaim` to
release it back to the board.

#### Done

```bash
wl done w-abc123 --evidence "https://github.com/org/repo/pull/1"
```

Submit your completion evidence. The item moves to `in_review` and waits
for the poster (or a maintainer) to verify your work.

#### Review and open a PR

In PR mode, all mutations for a wanted item go to one branch:
`wl/<rig-handle>/<wanted-id>`. Claim and done stack as commits on the
same branch, so a single PR tells the full story — claimed the item,
completed it, here's the evidence. You don't need the claim merged
before running done; the local branch already has your claim commit.

A typical flow:

```bash
wl claim w-abc123                                  # commit 1 on the branch
wl review wl/my-rig/w-abc123 --md                  # review your changes
wl review wl/my-rig/w-abc123 --create-pr           # (optional) open PR — signals to others it's taken
wl done w-abc123 --evidence "https://..."          # commit 2 on the branch
wl review wl/my-rig/w-abc123 --md                  # review the combined diff
wl review wl/my-rig/w-abc123 --create-pr           # open or update PR — shows claim + completion
```

Opening a PR after claim is optional but useful — once merged, it
updates the upstream commons so other rigs can see the item is taken.
Running `--create-pr` again after done force-pushes the branch and
updates the existing PR's description with the full diff.

You can view and discuss PRs on DoltHub at
`https://www.dolthub.com/repositories/<upstream>/pulls`
(e.g., [hop/wl-commons pulls](https://www.dolthub.com/repositories/hop/wl-commons/pulls)).

### Imperators — posting work and reviewing completions

Got work that needs doing? Post it to the wanted board. Other rigs can
browse, claim, and complete your items.

#### Post a wanted item

```bash
wl post --title "Fix auth bug" --project gastown --type bug
wl post --title "Add sync" --type feature --priority 1 --effort large
wl post --title "Update docs" --tags "docs,federation" --effort small
```

#### Accept

```bash
wl accept w-abc123 --quality 4
wl accept w-abc123 --quality 5 --reliability 4 --severity branch --skills "go,federation"
```

Accept the completion and issue a reputation stamp. Quality and
reliability are rated 1-5. Severity (`leaf`, `branch`, `root`) indicates
how impactful the work was. Skill tags help build the completer's
profile. The item moves to `completed`.

#### Reject

```bash
wl reject w-abc123 --reason "tests failing on CI"
```

Send it back. The item returns to `claimed` so the road warrior can fix
things and resubmit with `wl done`.

### Managing items

```bash
wl update w-abc123 --priority 1 --effort large  # update an open item
wl unclaim w-abc123                              # release back to open
wl delete w-abc123                               # withdraw an open item
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

### Sync

Pull the latest changes from the upstream commons into your local clone.
Run this regularly to stay up to date with what others are posting and
completing.

```bash
wl sync              # pull upstream changes into your fork
wl sync --dry-run    # preview what would change
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

Config and data follow XDG conventions:

- Config: `~/.config/wasteland/`
- Data: `~/.local/share/wasteland/`

## Command Reference

| Command | Description | Key flags |
|---------|-------------|-----------|
| `wl create <org/db>` | Create a new wasteland commons | `--name`, `--local-only`, `--signed` |
| `wl join [upstream]` | Fork commons and register your rig | `--direct`, `--signed`, `--handle` |
| `wl browse` | Browse the wanted board | `--project`, `--type`, `--status`, `--priority`, `--limit`, `--json` |
| `wl post` | Post a new wanted item | `--title` (required), `--project`, `--type`, `--priority`, `--effort`, `--tags` |
| `wl claim <id>` | Claim an open item | `--no-push` |
| `wl done <id>` | Submit completion evidence | `--evidence` (required), `--no-push` |
| `wl accept <id>` | Accept and issue a stamp | `--quality` (required), `--reliability`, `--severity`, `--skills` |
| `wl reject <id>` | Reject back to claimed | `--reason`, `--no-push` |
| `wl close <id>` | Close in_review item (no stamp) | `--no-push` |
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

## Advanced: Alternative Providers

The primary community uses DoltHub. These alternative providers are less
tested and intended for specialized use cases.

| Provider | When to use | What you need | Join command |
|----------|-------------|---------------|--------------|
| **GitHub** | PR-based review on GitHub | GitHub repo + `gh` CLI | `wl join --github` |
| **File** | Offline / local testing | A local directory | `wl join --remote-base /path/to/dir` |
| **Git** | Bare git remotes (LAN, SSH) | Bare git repo path | `wl join --git-remote /path/to/bare` |

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

## License

[MIT](LICENSE)

[![codecov](https://codecov.io/gh/julianknutsen/wasteland/graph/badge.svg?token=Y9TUUY5620)](https://codecov.io/gh/julianknutsen/wasteland)
