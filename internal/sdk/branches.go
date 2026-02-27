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

// cleanupBranch closes any associated PR and removes the branch.
// If branch deletion fails (e.g. RemoteDB where DOLT_BRANCH is unsupported),
// clears item data so DetectBranchOverrides no longer flags it as pending.
// Caller must hold c.mu.
func (c *Client) cleanupBranch(branch string) {
	if c.ClosePR != nil {
		_ = c.ClosePR(branch)
	}
	if err := c.db.DeleteBranch(branch); err == nil {
		_ = c.db.DeleteRemoteBranch(branch)
		return
	}
	// Branch deletion failed — clear item data from the branch instead.
	// This makes branchStatus="" so DetectBranchOverrides skips it.
	_ = clearBranchData(c.db, branch)
}

// DiscardBranch closes any associated PR and removes the mutation branch.
func (c *Client) DiscardBranch(branch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ClosePR != nil {
		_ = c.ClosePR(branch)
	}

	if err := c.db.DeleteBranch(branch); err == nil {
		_ = c.db.DeleteRemoteBranch(branch)
		return nil
	}

	// Branch deletion failed — clear item data from the branch instead.
	return clearBranchData(c.db, branch)
}

// clearBranchData removes item data from a branch so it no longer shows as
// pending. Used as fallback when branch deletion is not supported.
func clearBranchData(db commons.DB, branch string) error {
	wantedID := extractWantedID(branch)
	if wantedID == "" {
		return nil
	}
	esc := commons.EscapeSQL(wantedID)
	// DoltHub write API accepts only one statement per call.
	// Delete completions first (FK dependency), then the wanted item.
	_ = db.Exec(branch, "wl discard: "+wantedID, false,
		fmt.Sprintf("DELETE FROM completions WHERE wanted_id='%s'", esc))
	return db.Exec(branch, "wl discard: "+wantedID, false,
		fmt.Sprintf("DELETE FROM wanted WHERE id='%s'", esc))
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
