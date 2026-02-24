# Wasteland v0.1.0 Smoke Test Plan

Manual and agent-executable smoke tests for pre-release validation.
All tests use `--remote-base` (file provider) or `--local-only` — no DoltHub
account or network required.

## Prerequisites

```bash
make build
export PATH="$(pwd)/bin:$PATH"
wl version
```

---

## Test 1: wl create --local-only (basic schema init)

Create a local-only wasteland and verify schema is applied.

```bash
wl create testorg/test-commons --local-only
```

**Expected output contains:**
- `Creating wasteland testorg/test-commons...`
- `Initialized dolt database`
- `Applied commons schema v1.0`
- `Committed initial schema`
- `Created wasteland: testorg/test-commons`

**Verify schema** (run dolt from within the database directory):

```bash
cd "$XDG_DATA_HOME/wasteland/testorg/test-commons"
dolt sql -r csv -q "SHOW TABLES"
```

**Expected:** 7 tables: `_meta`, `badges`, `chain_meta`, `completions`, `rigs`, `stamps`, `wanted`

```bash
dolt sql -r csv -q "SELECT * FROM _meta"
cd "$REPO_ROOT"
```

**Expected:** single row: `schema_version,1.0` (no `wasteland_name` row)

---

## Test 2: wl create --name (custom wasteland name in _meta)

```bash
wl create testorg2/named-commons --local-only --name "My Cool Wasteland"
```

**Verify _meta:**

```bash
cd "$XDG_DATA_HOME/wasteland/testorg2/named-commons"
dolt sql -r csv -q "SELECT * FROM _meta ORDER BY \`key\`"
cd "$REPO_ROOT"
```

**Expected:** two rows: `schema_version,1.0` and `wasteland_name,My Cool Wasteland`

---

## Test 3: wl create already-exists error

Re-run create on the database from Test 1.

```bash
wl create testorg/test-commons --local-only
```

**Expected:** exits non-zero with error containing `already exists`

---

## Test 4: wl create invalid upstream error

```bash
wl create noslash --local-only
```

**Expected:** exits non-zero with error containing `invalid upstream path` and `expected format 'org/database'`

---

## Test 5: requireDolt() — missing dolt errors

Hide dolt from PATH by creating a restricted PATH containing only the `wl`
binary. This avoids issues with `grep -v` when dolt lives in a shared
directory like `/usr/local/bin`.

```bash
NODOLT_BIN=$(mktemp -d)
cp "$(which wl)" "$NODOLT_BIN/"
PATH_BAK="$PATH"
export PATH="$NODOLT_BIN"
```

**5a — wl create:**

```bash
wl create testorg3/test --local-only
```

**Expected:** exits non-zero with `dolt is not installed or not in PATH`

**5b — wl join:**

```bash
wl join someorg/somedb --remote-base /tmp/fake --fork-org fakeorg
```

**Expected:** exits non-zero with `dolt is not installed or not in PATH`

**Restore PATH:**

```bash
export PATH="$PATH_BAK"
rm -rf "$NODOLT_BIN"
```

Note: `wl post` is not tested here because it fails on config loading
("rig has not joined a wasteland") before reaching the `requireDolt()` check.
The dolt requirement for mutations is exercised via `wl create` and `wl join`.

---

## Test 6: Full lifecycle via file provider

Exercises join → post → claim → done → browse → status → sync → list using
local file remotes. Same code paths as DoltHub, different transport.

### Setup upstream

The file provider needs a proper dolt remote store (not just a `dolt init`
directory). Create a working directory, apply the schema, then push to a
remote store directory.

```bash
REMOTE_DIR=$(mktemp -d)
WORK_DIR=$(mktemp -d)
cd "$WORK_DIR"
dolt init
```

Apply schema and seed, then push to the remote store:

```bash
SCHEMA_SQL="$(cat "$REPO_ROOT/schema/commons.sql")
INSERT IGNORE INTO _meta (\`key\`, value) VALUES ('wasteland_name', 'Test Wasteland');
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('--allow-empty', '-m', 'Initialize wl-commons schema v1.0');"
dolt sql -q "$SCHEMA_SQL"
mkdir -p "$REMOTE_DIR/upstream/wl-commons"
dolt remote add store "file://$REMOTE_DIR/upstream/wl-commons"
dolt push store main
cd "$REPO_ROOT"
rm -rf "$WORK_DIR"
```

### 6a — Join

```bash
wl join upstream/wl-commons --remote-base "$REMOTE_DIR" --fork-org myrig
```

**Expected output contains:** `Joined wasteland`

### 6b — List

```bash
wl list
```

**Expected output contains:** `upstream/wl-commons`

### 6c — Browse (empty board)

```bash
wl browse
```

**Expected:** exits successfully (empty or no items)

### 6d — Post

```bash
wl post --title "First test item" --project test --type feature --no-push
```

**Expected output contains:** `Posted wanted item` and a `w-` prefixed ID

### 6e — Browse (after post)

```bash
wl browse
```

**Expected:** exits successfully. Note: the posted item may not appear in browse
since `--no-push` keeps it in the local fork only and browse clones from upstream.

### 6f — Claim

```bash
wl claim $ITEM_ID --no-push
```

**Expected output contains:** `Claimed`

### 6g — Done

```bash
wl done $ITEM_ID --evidence "tested manually" --no-push
```

**Expected output contains:** `Completion submitted` or `submitted`

### 6h — Status

```bash
wl status $ITEM_ID
```

**Expected output contains:** `in_review`

### 6i — Sync

```bash
wl sync --dry-run
```

**Expected:** exits successfully

### 6j — Config

```bash
wl config get mode
```

**Expected output contains:** `wild-west`

### Cleanup

```bash
rm -rf "$REMOTE_DIR"
```

---

## Test 7: wl create --help in command list

### 7a — create in Available Commands

```bash
wl --help
```

**Expected output contains:** `create`

### 7b — create help flags

```bash
wl create --help
```

**Expected output contains:** `--name`, `--local-only`, `--signed`

---

## Test 8: go install verification

```bash
go install github.com/julianknutsen/wasteland/cmd/wl@latest
```

**Expected:** exits successfully (requires tag to be pushed; use `@main` before tagging)

```bash
wl version
```

**Expected:** shows version info

---

## Test 9: Schema file sanity check

### 9a — 7 CREATE TABLE statements

```bash
grep -c "CREATE TABLE" "$REPO_ROOT/schema/commons.sql"
```

**Expected:** `7`

### 9b — No DOLT_ procedure calls

```bash
grep -c "DOLT_" "$REPO_ROOT/schema/commons.sql"
```

**Expected:** `0`

---

## Notes

- Tests 1–5 and 7–9 need no network or DoltHub credentials.
- Test 6 uses `--remote-base` (file provider) — same join/post/claim/done/sync
  paths as DoltHub, just `file://` URLs instead of `doltremoteapi`.
- Test 6 upstream setup requires pushing to a `file://` remote store (not just
  `dolt init`). This matches how the integration tests in
  `test/integration/offline/` create upstream stores.
- Test 8 requires the release tag to be pushed first; can verify with `@main`
  before tagging.
- When running `dolt sql` to verify database contents, `cd` into the database
  directory first (dolt does not support a `-d` flag for specifying the database
  path).
