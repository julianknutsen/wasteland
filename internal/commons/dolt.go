package commons

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DoltHubToken returns the DoltHub API token from the environment.
func DoltHubToken() string {
	return os.Getenv("DOLTHUB_TOKEN")
}

// DoltHubOrg returns the default DoltHub organization from the environment.
func DoltHubOrg() string {
	return os.Getenv("DOLTHUB_ORG")
}

// PushWithSync pushes the local main branch to both upstream and origin remotes.
// If a push is rejected (stale), it pulls to merge and retries.
// Returns an error if any remote push ultimately fails.
func PushWithSync(dbDir string, stdout io.Writer) error {
	var failures []string
	for _, remote := range []string{"upstream", "origin"} {
		if err := pushRemote(dbDir, remote); err != nil {
			fmt.Fprintf(stdout, "  Syncing with %s...\n", remote)
			if pullErr := pullRemote(dbDir, remote); pullErr != nil {
				fmt.Fprintf(stdout, "  warning: sync from %s failed: %v\n", remote, pullErr)
				failures = append(failures, remote)
				continue
			}
			if err := pushRemote(dbDir, remote); err != nil {
				fmt.Fprintf(stdout, "  warning: push to %s failed after sync: %v\n", remote, err)
				failures = append(failures, remote)
				continue
			}
		}
		fmt.Fprintf(stdout, "  Pushed to %s\n", remote)
	}
	if len(failures) > 0 {
		return fmt.Errorf("push failed for remotes: %s", strings.Join(failures, ", "))
	}
	return nil
}

func pushRemote(dbDir, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "push", remote, "main")
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt push %s main: %w (%s)", remote, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func pullRemote(dbDir, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "pull", remote, "main")
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt pull %s main: %w (%s)", remote, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// PullUpstream pulls the latest changes from the upstream remote.
func PullUpstream(dbDir string) error {
	return pullRemote(dbDir, "upstream")
}

// FetchRemote fetches the latest refs from a named remote without merging.
func FetchRemote(dbDir, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "fetch", remote)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt fetch %s: %w (%s)", remote, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// doltSQLScript executes a SQL script against a dolt database directory.
func doltSQLScript(dbDir, script string) error {
	tmpFile, err := os.CreateTemp("", "dolt-script-*.sql")
	if err != nil {
		return fmt.Errorf("creating temp SQL file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing SQL script: %w", err)
	}
	_ = tmpFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dolt", "sql", "--file", tmpFile.Name())
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// BranchName returns the conventional branch name for a PR-mode mutation.
func BranchName(rigHandle, wantedID string) string {
	return fmt.Sprintf("wl/%s/%s", rigHandle, wantedID)
}

// BranchExists checks whether a branch exists in the dolt database.
func BranchExists(dbDir, branch string) (bool, error) {
	out, err := DoltSQLQuery(dbDir, fmt.Sprintf(
		"SELECT COUNT(*) AS cnt FROM dolt_branches WHERE name = '%s'",
		strings.ReplaceAll(branch, "'", "''"),
	))
	if err != nil {
		return false, err
	}
	// CSV output: "cnt\n0\n" or "cnt\n1\n"
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return false, fmt.Errorf("unexpected dolt_branches output: %s", out)
	}
	return strings.TrimSpace(lines[1]) != "0", nil
}

// CheckoutBranch creates the branch if it doesn't exist, then checks it out.
// Uses dolt CLI commands (not SQL DOLT_CHECKOUT) because the SQL stored
// procedure is session-scoped and does not persist across dolt sql invocations.
func CheckoutBranch(dbDir, branch string) error {
	exists, err := BranchExists(dbDir, branch)
	if err != nil {
		return fmt.Errorf("checking branch %s: %w", branch, err)
	}
	if !exists {
		if err := doltExec(dbDir, "branch", branch); err != nil {
			return fmt.Errorf("creating branch %s: %w", branch, err)
		}
	}
	return doltExec(dbDir, "checkout", branch)
}

// CheckoutMain switches the working directory back to the main branch.
func CheckoutMain(dbDir string) error {
	return doltExec(dbDir, "checkout", "main")
}

// doltExec runs a dolt CLI command in the given database directory.
func doltExec(dbDir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", args...)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

// PushBranch force-pushes a named branch to origin.
// Force is always used because wl/* branches on the user's own fork may
// have diverged history after redo operations (unclaim then re-claim, etc.).
func PushBranch(dbDir, branch string, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "push", "--force", "origin", branch)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(stdout, "  warning: push branch %s to origin failed: %v (%s)\n", branch, err, strings.TrimSpace(string(output)))
		return fmt.Errorf("push branch %s: %w", branch, err)
	}
	fmt.Fprintf(stdout, "  Pushed branch %s to origin\n", branch)
	return nil
}

// ListBranches returns branch names matching a prefix (e.g. "wl/").
func ListBranches(dbDir, prefix string) ([]string, error) {
	out, err := DoltSQLQuery(dbDir, fmt.Sprintf(
		"SELECT name FROM dolt_branches WHERE name LIKE '%s%%' ORDER BY name",
		strings.ReplaceAll(prefix, "'", "''"),
	))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil, nil // header only, no branches
	}
	var branches []string
	for _, line := range lines[1:] {
		name := strings.TrimSpace(line)
		if name != "" {
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// MergeBranch merges a branch into main. If the merge produces conflicts
// it aborts and returns an error. The caller must already be on main.
func MergeBranch(dbDir, branch string) error {
	escaped := strings.ReplaceAll(branch, "'", "''")
	err := doltSQLScript(dbDir, fmt.Sprintf(
		"CALL DOLT_CHECKOUT('main');\nCALL DOLT_MERGE('%s');", escaped,
	))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "conflict") {
			_ = doltSQLScript(dbDir, "CALL DOLT_MERGE('--abort');")
			return fmt.Errorf("merge conflict on branch %s: resolve manually or delete the branch", branch)
		}
		return fmt.Errorf("merging branch %s: %w", branch, err)
	}
	return nil
}

// DeleteBranch deletes a local branch.
func DeleteBranch(dbDir, branch string) error {
	escaped := strings.ReplaceAll(branch, "'", "''")
	return doltSQLScript(dbDir, fmt.Sprintf("CALL DOLT_BRANCH('-D', '%s');", escaped))
}

// EnsureGitHubRemote adds a "github" Dolt remote pointing to the rig's
// GitHub fork (e.g. https://github.com/alice-dev/wl-commons.git).
// Idempotent: if "github" remote already exists, no-op.
func EnsureGitHubRemote(dbDir, forkOrg, forkDB string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	checkCmd := exec.CommandContext(ctx, "dolt", "remote", "-v")
	checkCmd.Dir = dbDir
	output, err := checkCmd.CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "github") {
				return nil
			}
		}
	}

	remoteURL := fmt.Sprintf("https://github.com/%s/%s.git", forkOrg, forkDB)
	addCmd := exec.CommandContext(ctx, "dolt", "remote", "add", "github", remoteURL)
	addCmd.Dir = dbDir
	output, err = addCmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if strings.Contains(strings.ToLower(msg), "already exists") {
			return nil
		}
		return fmt.Errorf("dolt remote add github: %w (%s)", err, msg)
	}
	return nil
}

// PushBranchToRemote pushes a branch to a named remote.
func PushBranchToRemote(dbDir, remote, branch string, stdout io.Writer) error {
	return PushBranchToRemoteForce(dbDir, remote, branch, false, stdout)
}

// PushBranchToRemoteForce pushes a branch to a named remote, optionally with --force.
func PushBranchToRemoteForce(dbDir, remote, branch string, force bool, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"push"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, remote, branch)
	cmd := exec.CommandContext(ctx, "dolt", args...)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt push %s %s: %w (%s)", remote, branch, err, strings.TrimSpace(string(output)))
	}
	fmt.Fprintf(stdout, "  Pushed branch %s to %s\n", branch, remote)
	return nil
}

// ListWantedIDs returns wanted item IDs, optionally filtered by status.
func ListWantedIDs(dbDir, statusFilter string) ([]string, error) {
	query := "SELECT id FROM wanted"
	if statusFilter != "" {
		query += fmt.Sprintf(" WHERE status = '%s'", EscapeSQL(statusFilter))
	}
	query += " ORDER BY created_at DESC LIMIT 50"
	out, err := DoltSQLQuery(dbDir, query)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil, nil
	}
	var ids []string
	for _, line := range lines[1:] {
		id := strings.TrimSpace(line)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ResolveWantedID resolves a wanted ID or unambiguous prefix to a full ID.
func ResolveWantedID(dbDir, idOrPrefix string) (string, error) {
	query := fmt.Sprintf("SELECT id FROM wanted WHERE id LIKE '%s%%' LIMIT 3", EscapeSQL(idOrPrefix))
	out, err := DoltSQLQuery(dbDir, query)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("no wanted item matching %q", idOrPrefix)
	}
	var matches []string
	for _, line := range lines[1:] {
		id := strings.TrimSpace(line)
		if id != "" {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no wanted item matching %q", idOrPrefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous prefix %q matches: %s", idOrPrefix, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

// DoltSQLQuery executes a SQL query and returns the raw CSV output.
func DoltSQLQuery(dbDir, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dolt", "sql", "-r", "csv", "-q", query)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("dolt sql query failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
