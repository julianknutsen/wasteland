package sdk

import (
	"fmt"
	"io"
	"strings"

	"github.com/julianknutsen/wasteland/internal/commons"
)

// ApplyBranch merges a mutation branch into main, deletes the branch, and pushes.
func (c *Client) ApplyBranch(branch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.db.MergeBranch(branch); err != nil {
		return err
	}
	if err := c.db.DeleteBranch(branch); err != nil {
		return fmt.Errorf("delete local branch: %w", err)
	}
	if err := c.db.PushMain(io.Discard); err != nil {
		return fmt.Errorf("push origin main: %w", err)
	}
	return nil
}

// cleanupBranch closes any associated PR, clears item data, and deletes the branch.
// Caller must hold c.mu.
func (c *Client) cleanupBranch(branch string) {
	if c.ClosePR != nil {
		_ = c.ClosePR(branch)
	}
	if wantedID := extractWantedID(branch); wantedID != "" {
		esc := commons.EscapeSQL(wantedID)
		_ = c.db.Exec(branch, "wl discard: "+wantedID, false,
			fmt.Sprintf("DELETE FROM completions WHERE wanted_id='%s'", esc),
			fmt.Sprintf("DELETE FROM wanted WHERE id='%s'", esc))
	}
	_ = c.db.DeleteBranch(branch)
	_ = c.db.DeleteRemoteBranch(branch)
}

// DiscardBranch closes any associated PR and deletes the mutation branch.
func (c *Client) DiscardBranch(branch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanupBranch(branch)
	return nil
}

// extractWantedID pulls the wanted ID from a branch name like "wl/{rig}/{wantedID}".
func extractWantedID(branch string) string {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) == 3 && parts[0] == "wl" {
		return parts[2]
	}
	return ""
}

// SubmitPR creates a pull request for the given branch.
func (c *Client) SubmitPR(branch string) (string, error) {
	if c.CreatePR == nil {
		return "", fmt.Errorf("PR creation not available")
	}
	return c.CreatePR(branch)
}

// BranchDiff returns a diff for the given branch.
func (c *Client) BranchDiff(branch string) (string, error) {
	if c.LoadDiff == nil {
		return "", fmt.Errorf("diff loading not available")
	}
	return c.LoadDiff(branch)
}

// SaveSettings persists mode and signing settings, updating the client's state.
func (c *Client) SaveSettings(mode string, signing bool) error {
	if c.SaveConfig == nil {
		return fmt.Errorf("settings persistence not available")
	}
	if err := c.SaveConfig(mode, signing); err != nil {
		return err
	}
	c.mode = mode
	c.signing = signing
	return nil
}

// Sync pulls the latest data from upstream.
func (c *Client) Sync() error {
	return c.db.Sync()
}
