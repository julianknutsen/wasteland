//go:build integration

package offline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setMode updates the wasteland config to the given mode.
func setMode(t *testing.T, env *testEnv, upstreamPath, mode string) {
	t.Helper()
	cfg := env.loadConfig(t, upstreamPath)
	cfg["mode"] = mode

	parts := strings.SplitN(upstreamPath, "/", 2)
	configPath := filepath.Join(env.ConfigDir, "wastelands", parts[0], parts[1]+".json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshaling config: %v", err)
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

func TestPRModePost(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)
			dbDir := forkCloneDir(t, env)

			// Switch to PR mode.
			setMode(t, env, upstream, "pr")

			// Post in PR mode.
			stdout, stderr, err := runWL(t, env, "post",
				"--title", "PR mode test",
				"--type", "feature",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			if !strings.Contains(stdout, "Branch:") {
				t.Errorf("expected Branch output in PR mode, got: %s", stdout)
			}

			wantedID := extractWantedID(t, stdout)

			// Verify a branch was created.
			expectedBranch := "wl/" + forkOrg + "/" + wantedID
			raw := doltSQL(t, dbDir, "SELECT name FROM dolt_branches WHERE name='"+expectedBranch+"'")
			rows := parseCSV(t, raw)
			if len(rows) < 2 {
				t.Errorf("expected branch %q to exist", expectedBranch)
			}
		})
	}
}

func TestPRModeClaim(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)
			dbDir := forkCloneDir(t, env)

			// Post in wild-west mode.
			stdout, _, err := runWL(t, env, "post",
				"--title", "PR claim test",
				"--type", "bug",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v", err)
			}
			wantedID := extractWantedID(t, stdout)

			// Switch to PR mode.
			setMode(t, env, upstream, "pr")

			// Claim in PR mode.
			stdout, stderr, err := runWL(t, env, "claim", wantedID, "--no-push")
			if err != nil {
				t.Fatalf("wl claim failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			if !strings.Contains(stdout, "Branch:") {
				t.Errorf("expected Branch output in PR mode, got: %s", stdout)
			}

			// Verify a branch was created.
			expectedBranch := "wl/" + forkOrg + "/" + wantedID
			raw := doltSQL(t, dbDir, "SELECT name FROM dolt_branches WHERE name='"+expectedBranch+"'")
			rows := parseCSV(t, raw)
			if len(rows) < 2 {
				t.Errorf("expected branch %q to exist", expectedBranch)
			}
		})
	}
}

func TestPRModeReturnToMain(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)
			dbDir := forkCloneDir(t, env)

			// Switch to PR mode.
			setMode(t, env, upstream, "pr")

			// Post.
			stdout, _, err := runWL(t, env, "post",
				"--title", "Return to main test",
				"--type", "feature",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v", err)
			}
			_ = extractWantedID(t, stdout)

			// Active branch should be main after the command completes.
			raw := doltSQL(t, dbDir, "SELECT active_branch()")
			rows := parseCSV(t, raw)
			if len(rows) < 2 {
				t.Fatal("could not query active branch")
			}
			branch := strings.TrimSpace(rows[1][0])
			if branch != "main" {
				t.Errorf("active branch = %q, want %q", branch, "main")
			}
		})
	}
}

func TestWildWestModeUnchanged(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)
			dbDir := forkCloneDir(t, env)

			// Post in default (wild-west) mode.
			stdout, _, err := runWL(t, env, "post",
				"--title", "Wild-west unchanged test",
				"--type", "feature",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v", err)
			}

			if strings.Contains(stdout, "Branch:") {
				t.Errorf("wild-west mode should not print Branch line, got: %s", stdout)
			}

			wantedID := extractWantedID(t, stdout)

			// No wl/* branch should have been created.
			raw := doltSQL(t, dbDir, "SELECT COUNT(*) FROM dolt_branches WHERE name LIKE 'wl/%'")
			rows := parseCSV(t, raw)
			if len(rows) < 2 || strings.TrimSpace(rows[1][0]) != "0" {
				t.Errorf("expected 0 wl/* branches in wild-west mode, got: %v", rows)
			}

			// Item should exist on main.
			raw = doltSQL(t, dbDir, "SELECT id FROM wanted WHERE id='"+wantedID+"'")
			rows = parseCSV(t, raw)
			if len(rows) < 2 {
				t.Errorf("wanted item %s should exist on main", wantedID)
			}
		})
	}
}

func TestReviewListBranches(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			// Switch to PR mode and post.
			setMode(t, env, upstream, "pr")

			stdout, _, err := runWL(t, env, "post",
				"--title", "Review list test",
				"--type", "feature",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v", err)
			}
			wantedID := extractWantedID(t, stdout)

			// wl review (no args) should list the branch.
			stdout, stderr, err := runWL(t, env, "review")
			if err != nil {
				t.Fatalf("wl review failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			expectedBranch := "wl/" + forkOrg + "/" + wantedID
			if !strings.Contains(stdout, expectedBranch) {
				t.Errorf("expected review to list %q, got: %s", expectedBranch, stdout)
			}
		})
	}
}

func TestReviewShowsDiff(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			// Switch to PR mode and post.
			setMode(t, env, upstream, "pr")

			stdout, _, err := runWL(t, env, "post",
				"--title", "Review diff test",
				"--type", "feature",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v", err)
			}
			wantedID := extractWantedID(t, stdout)

			// wl review <branch> --stat should show diff.
			branch := "wl/" + forkOrg + "/" + wantedID
			stdout, stderr, err := runWL(t, env, "review", branch, "--stat")
			if err != nil {
				t.Fatalf("wl review --stat failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			if !strings.Contains(stdout, "wanted") {
				t.Errorf("expected diff stat to mention 'wanted' table, got: %s", stdout)
			}
		})
	}
}

func TestConfigSetGetMode(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			// Default mode should be wild-west.
			stdout, stderr, err := runWL(t, env, "config", "get", "mode")
			if err != nil {
				t.Fatalf("wl config get mode failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			if strings.TrimSpace(stdout) != "wild-west" {
				t.Errorf("default mode = %q, want %q", strings.TrimSpace(stdout), "wild-west")
			}

			// Set to PR mode.
			stdout, stderr, err = runWL(t, env, "config", "set", "mode", "pr")
			if err != nil {
				t.Fatalf("wl config set mode pr failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			// Verify it's set.
			stdout, stderr, err = runWL(t, env, "config", "get", "mode")
			if err != nil {
				t.Fatalf("wl config get mode failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			if strings.TrimSpace(stdout) != "pr" {
				t.Errorf("mode = %q, want %q", strings.TrimSpace(stdout), "pr")
			}

			// Set back to wild-west.
			stdout, stderr, err = runWL(t, env, "config", "set", "mode", "wild-west")
			if err != nil {
				t.Fatalf("wl config set mode wild-west failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			// Verify.
			stdout, _, err = runWL(t, env, "config", "get", "mode")
			if err != nil {
				t.Fatalf("wl config get mode failed: %v", err)
			}
			if strings.TrimSpace(stdout) != "wild-west" {
				t.Errorf("mode = %q, want %q", strings.TrimSpace(stdout), "wild-west")
			}
		})
	}
}
