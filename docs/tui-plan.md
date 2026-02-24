# TUI Implementation Plan

## Overview

Full-screen interactive TUI (`wl tui`) using Bubbletea. Lets users browse
the board, inspect items, and take actions without leaving the terminal.

Built on top of the lifecycle state machine (`internal/commons/lifecycle.go`)
so the TUI and CLI share one source of truth for valid transitions.

## Design Decisions

1. **Bubbletea + Bubbles** — Charm ecosystem, same as existing lipgloss dep.
   Elm architecture (Model/Update/View) fits naturally.

2. **SDK-first queries** — Extract browse/dashboard SQL from CLI command
   handlers into `internal/commons/` as named functions. Both CLI and TUI
   call the same functions. End-state: a commons SDK usable by web UIs too.

3. **Dashboard vs Browse are separate views** — Dashboard = "my work"
   (claimed, awaiting review, completions). Browse = "find new work"
   (filterable board). Different purposes, different queries.

4. **Sub-models are concrete structs** — All views known at compile time.
   Root model owns them as fields, delegates Update/View based on activeView.

5. **Navigation via messages** — Sub-models return `navigateMsg` via Cmds.
   Root model orchestrates transitions + data fetching.

6. **All I/O via async Cmds** — Every dolt query wrapped in a `tea.Cmd`.
   Event loop never blocks.

7. **Action overlay is a separate model** — Modal drawn with `lipgloss.Place()`
   on top of detail view. Manages its own input/confirmation/errors.

## Lifecycle Integration

- `ValidateTransition()` determines which action keys to show in detail view
- `DetectItemLocation()` powers statusbar sync indicator
- `ResolvePushTarget()` lets action overlay explain what push will do
- `refreshPR()` auto-updates PR descriptions after TUI mutations

---

## Phase 1: Browse + Detail (read-only)

Proves bubbletea integration. Users can browse the board and inspect items.

### New dependencies

```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
```

### Files

| File | Purpose |
|------|---------|
| `internal/commons/queries.go` | Extract BrowseFilter + BrowseWanted() from cmd_browse, QueryWantedSummary for lists |
| `internal/tui/tui.go` | Root model: Init/Update/View, view routing |
| `internal/tui/messages.go` | All message types (data loaded, navigate, error, resize) |
| `internal/tui/keys.go` | Key bindings via bubbles/key |
| `internal/tui/theme.go` | Ayu colors adapted for TUI (borders, selection, etc.) |
| `internal/tui/browse.go` | Browse view: scrollable filtered item list |
| `internal/tui/detail.go` | Detail view: viewport with full item info |
| `internal/tui/statusbar.go` | Bottom bar: handle, sync status, contextual key hints |
| `cmd/wl/cmd_tui.go` | Cobra subcommand, launches tea.Program |

### Modified files

| File | Change |
|------|--------|
| `cmd/wl/main.go` | Register `tui` subcommand |
| `cmd/wl/cmd_browse.go` | Use `commons.BrowseWanted()` instead of inline SQL |
| `go.mod` / `go.sum` | Add bubbletea + bubbles |

### Browse view

```
Wasteland Board (12 open)          /: search  s: status  t: type  q: quit
-----------------------------------------------------------------------
  ID           TITLE                     PROJECT  TYPE   PRI  EFFORT
> w-abc123     Fix authentication flow   gastown  bug    P0   medium
  w-def456     Add REST API docs         gastown  docs   P2   small
  w-ghi789     Design badge system       beads    design P1   large
  ...
-----------------------------------------------------------------------
alice@steveyegge/wl-commons                                   1:Dash 2:Board
```

- `j/k` or arrows: move cursor
- `/`: toggle search input (filters title in real-time)
- `s`: cycle status filter (open -> claimed -> in_review -> all)
- `t`: cycle type filter (all -> feature -> bug -> design -> ...)
- `Enter`: open detail view
- `q`: quit

### Detail view

```
w-abc123: Fix authentication flow
-----------------------------------------------------------------------
  Status:     claimed              Priority: P0
  Type:       bug                  Effort:   medium
  Project:    gastown              Posted by: bob
  Claimed by: alice
  Tags:       auth, security
  Created:    2026-02-10           Updated:  2026-02-20

  Description:
    The authentication flow breaks when...
    (scrollable viewport for long descriptions)

  Completion:  c-abc123def456
    Evidence:    https://github.com/org/repo/pull/123
    Completed by: alice
-----------------------------------------------------------------------
Esc: back                                       c:claim d:done a:accept
```

- `j/k`: scroll viewport
- `Esc`: back to browse
- Action keys shown but disabled in Phase 1 (grayed out hint)

---

## Phase 2: Dashboard

Adds the "my work" view as the landing page.

### Files

| File | Purpose |
|------|---------|
| `internal/commons/queries.go` | Add MyClaimedItems(), MyReviewItems(), MyCompletedItems() |
| `internal/tui/dashboard.go` | Dashboard view: sections for claimed, review, completions |

### Dashboard view

```
Wasteland Dashboard                                          [synced]
-----------------------------------------------------------------------
  Claimed (2):
  > w-abc123   Fix auth flow        claimed  P1  medium
    w-def456   Add API docs         claimed  P2  small

  Awaiting Review (1):
    w-ghi789   Refactor parser      in_review  P0  large

  Recent Completions (1):
    w-jkl012   Setup CI pipeline    completed
-----------------------------------------------------------------------
alice@steveyegge/wl-commons        Tab: section  Enter: detail  2: Board
```

- `j/k`: move within section
- `Tab/Shift+Tab`: switch sections
- `Enter`: open detail
- `2`: switch to Browse

---

## Phase 3: Mutations (action overlay)

Adds claim/unclaim/done/accept/reject/close via modal overlays.

### Files

| File | Purpose |
|------|---------|
| `internal/tui/action.go` | Action modal: confirmation, text input, multi-step forms |
| `internal/tui/action_test.go` | Tests for action state machine |

### Action types

| Action | Modal content |
|--------|--------------|
| claim / unclaim / close / delete | Simple confirmation ("Claim w-abc123?") |
| done | Text input for evidence URL |
| reject | Text input for reason |
| accept | Multi-step: quality (1-5), severity, skills, message |

### Flow

1. User presses action key in detail view (e.g. `c` for claim)
2. `ValidateTransition()` checks if action is valid for current status
3. Action modal overlays detail view
4. User confirms -> async Cmd runs mutation via commons store
5. On success: refresh detail, show result, `refreshPR()` if branch push
6. On error: show error in modal, allow retry or dismiss

---

## Phase 4: Post/Update forms

Full-screen forms for creating and editing wanted items.

### Files

| File | Purpose |
|------|---------|
| `internal/tui/form.go` | Multi-field form (title, description, project, type, priority, effort, tags) |
| `internal/tui/form_test.go` | Tests |

- `n` from browse: new post form
- `e` from detail: edit form (pre-filled)

---

## Phase 5: Real-time sync

Background polling + push notifications in the TUI session.

- Periodic `dolt fetch upstream` on a timer (configurable interval)
- Statusbar shows "synced", "fetching...", "N items changed"
- Optional: detect upstream changes and refresh current view

---

## Implementation Order

Phase 1 is the priority. Ship it, get feedback, iterate.

Each phase is a separate PR. Phases 2-5 can be reordered based on
what users actually ask for after Phase 1 ships.
