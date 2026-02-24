---
name: smoke-test
description: Run the wasteland smoke test plan against a freshly built wl binary
---

# Smoke Test Runner

Execute the smoke tests defined in `test/smoke/TESTPLAN.md`.

## Instructions

1. **Read the test plan.** Read `test/smoke/TESTPLAN.md` from the repo root. Parse
   each numbered `## Test N:` section. Each section has shell commands in fenced
   code blocks and expected output markers in **Expected** lines.

2. **Build.** Run `make build` from the repo root. Fail immediately if the build
   fails.

3. **Set up PATH.** Put `bin/` (from the repo root) at the front of `PATH` so
   the freshly built `wl` binary is used.

4. **Set `REPO_ROOT`.** Export `REPO_ROOT` pointing to the repository root so
   test commands that reference `$REPO_ROOT` (e.g., schema file paths) resolve
   correctly.

5. **Isolate each test.** For every test section, create a fresh temp directory
   and export:
   - `XDG_DATA_HOME=$tmpdir/data`
   - `XDG_CONFIG_HOME=$tmpdir/config`
   - `HOME=$tmpdir/home`
   - `DOLT_ROOT_PATH=$tmpdir/home`

   Create `$HOME/.dolt/config_global.json` with:
   ```json
   {"user.name":"smoke-test","user.email":"smoke@test.local","user.creds":""}
   ```

   This ensures real user config/data is never touched and each test starts
   from a clean slate.

6. **Execute tests sequentially.** For each `## Test N:` section:
   - Run each fenced code block as a bash command.
   - For blocks preceded by an **Expected** marker, check:
     - **"exits non-zero"**: the command must return a non-zero exit code.
     - **"exits successfully"**: the command must return exit code 0.
     - **"contains: `<text>`"** or **"Expected output contains: `<text>`"**: stdout+stderr
       must contain the specified text (case-sensitive substring match).
     - **"Expected: `N`"** (a bare number): stdout must contain that exact value
       (used for `grep -c` checks).
   - If `$ITEM_ID` is referenced, capture it from the `wl post` output by
     extracting the `w-<hex>` pattern (regex: `w-[0-9a-f]+`).
   - Print `PASS: Test N — <title>` on success.
   - Print `FAIL: Test N — <title>` on failure with the actual output, then
     **stop immediately** (do not continue to later tests).

7. **Skip Test 8** (`go install` verification) — it requires a pushed tag and
   network access. Print `SKIP: Test 8 — go install verification`.

8. **Print summary.** After all tests, print a summary line:
   ```
   Smoke tests: X passed, Y failed, Z skipped
   ```

9. **Clean up.** Remove all temp directories created during the run.

## Important details

- Do NOT hardcode test content — always read `test/smoke/TESTPLAN.md` as the
  source of truth. If the plan file changes, the tests change automatically.
- The file provider (`--remote-base`) uses local filesystem paths, no network.
- Test 5 hides dolt from PATH by creating a temp bin directory containing only
  the `wl` binary and setting `PATH` to that single directory. Do NOT use
  `grep -v dolt` to filter PATH — dolt is typically in a shared directory like
  `/usr/local/bin` that won't be filtered by name.
- Test 6 upstream setup requires a proper dolt remote store: init a working
  directory, apply schema via `DOLT_ADD`/`DOLT_COMMIT` stored procedures, then
  push to a `file://` store directory. A bare `dolt init` directory is NOT a
  valid remote store.
- When running `dolt sql` to verify database contents, `cd` into the database
  directory first — dolt does not support a `-d` flag. Use `-r csv` (not
  `--result-format csv`) for output format.
- Test 6 requires `dolt` on PATH. If dolt is not available, fail with a clear
  message rather than producing confusing errors.
- Use `--no-push` on mutation commands (post, claim, done) in Test 6 since the
  file provider may not support push in all configurations.
- `dolt init` needs `HOME` and `DOLT_ROOT_PATH` set to the temp home dir, or
  it will try to write to the real home directory.
- Items posted with `--no-push` stay in the local fork only. `wl browse` clones
  from upstream, so posted items may not appear in browse output. This is correct
  behavior — verify the post via its `ITEM_ID` using `wl status` instead.
