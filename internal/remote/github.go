package remote

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitHubProvider implements Provider using GitHub repositories as dolt remotes.
// Dolt can push to and clone from GitHub repos via https:// URLs.
type GitHubProvider struct{}

// NewGitHubProvider creates a GitHubProvider.
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

// DatabaseURL returns the GitHub HTTPS URL for org/db.
func (g *GitHubProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("https://github.com/%s/%s.git", org, db)
}

// Fork creates a fork of fromOrg/fromDB under toOrg on GitHub using the gh CLI.
// If the fork already exists, this is a no-op (gh handles idempotency).
func (g *GitHubProvider) Fork(fromOrg, fromDB, toOrg string) error {
	sourceRepo := fromOrg + "/" + fromDB
	cmd := exec.Command("gh", "repo", "fork", sourceRepo, "--org", toOrg, "--clone=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		// gh reports "already exists" when fork is present — treat as success.
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "already exists") {
			return nil
		}
		return fmt.Errorf("gh repo fork %s --org %s: %w (%s)", sourceRepo, toOrg, err, msg)
	}
	return nil
}

// Type returns "github".
func (g *GitHubProvider) Type() string { return "github" }
