package remote

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitProvider implements Provider using bare git repositories as dolt remotes.
// Dolt can push to and clone from bare git repos via file:// URLs.
//
// The baseDir contains bare git repos at {org}/{db}.git. These are
// initialized with "git init --bare" and populated by "dolt push".
type GitProvider struct {
	baseDir string
}

// NewGitProvider creates a GitProvider rooted at baseDir.
func NewGitProvider(baseDir string) *GitProvider {
	return &GitProvider{baseDir: baseDir}
}

// DatabaseURL returns the file:// URL for the bare git repo at org/db.git.
func (g *GitProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("file://%s", filepath.Join(g.baseDir, org, db+".git"))
}

// Fork clones the source and pushes to a new bare git repo under toOrg.
func (g *GitProvider) Fork(fromOrg, fromDB, toOrg string) error {
	srcURL := g.DatabaseURL(fromOrg, fromDB)
	destPath := filepath.Join(g.baseDir, toOrg, fromDB+".git")
	destURL := g.DatabaseURL(toOrg, fromDB)

	// Already forked?
	if info, err := os.Stat(destPath); err == nil && info.IsDir() {
		return nil
	}

	// Clone from source into a temp dolt working directory.
	tmpDir, err := os.MkdirTemp("", "dolt-git-fork-*")
	if err != nil {
		return fmt.Errorf("creating temp dir for fork: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workDir := filepath.Join(tmpDir, "work")
	cmd := exec.Command("dolt", "clone", srcURL, workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloning source for fork: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	// Initialize a bare git repo as the destination store.
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("creating fork git repo dir: %w", err)
	}

	cmd = exec.Command("git", "init", "--bare", destPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init --bare %s: %w (%s)", destPath, err, strings.TrimSpace(string(output)))
	}

	// Seed the bare repo with an initial commit so dolt can push to it.
	// Without this, dolt refuses to push to a bare repo that has no branches.
	if err := seedBareGitRepo(destPath); err != nil {
		return fmt.Errorf("seeding bare repo: %w", err)
	}

	// Push from the temp working dir to the new bare repo.
	cmd = exec.Command("dolt", "remote", "add", "fork-dest", destURL)
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adding fork dest remote: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	cmd = exec.Command("dolt", "push", "fork-dest", "main")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pushing to fork dest: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// CreatePR is a no-op for git providers (no PR support).
func (g *GitProvider) CreatePR(_, _, _, _, _, _ string) (string, error) { return "", nil }

// Type returns "git".
func (g *GitProvider) Type() string { return "git" }

// seedBareGitRepo creates an initial empty commit in a bare git repo
// so dolt can push to it. Without this, dolt may refuse to push to a
// bare repo that has no branches.
func seedBareGitRepo(bareDir string) error {
	tmpDir, err := os.MkdirTemp("", "git-seed-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cmd := exec.Command("git", "init", "-b", "main", tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	cmd = exec.Command("git", "-C", tmpDir,
		"-c", "user.name=init", "-c", "user.email=init@init",
		"commit", "--allow-empty", "-m", "init")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	cmd = exec.Command("git", "-C", tmpDir, "push", "file://"+bareDir, "main")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}
