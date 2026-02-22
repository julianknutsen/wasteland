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

func (g *GitProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("file://%s", filepath.Join(g.baseDir, org, db+".git"))
}

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
	defer os.RemoveAll(tmpDir)

	workDir := filepath.Join(tmpDir, "work")
	cmd := exec.Command("dolt", "clone", srcURL, workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloning source for fork: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	// Initialize a bare git repo as the destination store.
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("creating fork git repo dir: %w", err)
	}

	cmd = exec.Command("git", "init", "--bare", destPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init --bare %s: %w (%s)", destPath, err, strings.TrimSpace(string(output)))
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

func (g *GitProvider) Type() string { return "git" }
