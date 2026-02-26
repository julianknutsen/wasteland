package sdk

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/julianknutsen/wasteland/internal/commons"
)

// MutationResult holds the outcome of a mutation operation.
type MutationResult struct {
	Detail *DetailResult
	Branch string // mutation branch name (PR mode) or ""
	Hint   string // user-facing hint ("" if none)
}

// mutate is the internal mode-aware mutation helper.
// Wild-west: exec DML on main → push → refresh detail.
// PR: exec DML on branch → read branch state → push branch → auto-cleanup if reverted.
func (c *Client) mutate(wantedID, commitMsg string, stmts ...string) (*MutationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mode == "pr" {
		return c.mutatePR(wantedID, commitMsg, stmts...)
	}
	return c.mutateWildWest(wantedID, commitMsg, stmts...)
}

func (c *Client) mutateWildWest(wantedID, commitMsg string, stmts ...string) (*MutationResult, error) {
	if err := c.db.Exec("", commitMsg, c.signing, stmts...); err != nil {
		return nil, err
	}
	if err := c.db.PushWithSync(io.Discard); err != nil {
		return nil, err
	}
	detail, err := c.detailWildWest(wantedID)
	if err != nil {
		return nil, err
	}
	return &MutationResult{Detail: detail}, nil
}

func (c *Client) mutatePR(wantedID, commitMsg string, stmts ...string) (*MutationResult, error) {
	branch := commons.BranchName(c.rigHandle, wantedID)
	mainStatus, _, _ := commons.QueryItemStatus(c.db, wantedID, "main")

	if err := c.db.Exec(branch, commitMsg, c.signing, stmts...); err != nil {
		return nil, err
	}

	// Read post-mutation state from branch.
	item, completion, stamp, _ := commons.QueryFullDetailAsOf(c.db, wantedID, branch)

	detail := &DetailResult{
		Item:       item,
		Completion: completion,
		Stamp:      stamp,
		Branch:     branch,
		MainStatus: mainStatus,
	}
	if item != nil {
		detail.Actions = commons.AvailableTransitions(item, c.rigHandle)
		switch {
		case mainStatus == "":
			detail.Delta = "new"
		case item.Status != mainStatus:
			detail.Delta = commons.DeltaLabel(mainStatus, item.Status)
		default:
			detail.Delta = "changes"
		}
	}
	if branch != "" && c.CheckPR != nil {
		detail.PRURL = c.CheckPR(branch)
	}
	if branch != "" && c.BranchURL != nil {
		detail.BranchURL = c.BranchURL(branch)
	}
	detail.BranchActions = c.computeBranchActions(detail)

	// Push the branch.
	var pushLog bytes.Buffer
	if err := c.db.PushBranch(branch, &pushLog); err != nil {
		if msg := strings.TrimSpace(pushLog.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("push branch: %w", err)
	}

	result := &MutationResult{Detail: detail, Branch: branch}

	// Auto-cleanup: if mutation reverted item to main status, delete the branch.
	if mainStatus != "" && item != nil && item.Status == mainStatus {
		_ = c.db.DeleteBranch(branch)
		_ = c.db.DeleteRemoteBranch(branch)
		detail.Branch = ""
		detail.MainStatus = ""
		detail.Delta = ""
		result.Branch = ""
		result.Hint = "reverted — branch cleaned up"
	}

	// Auto-submit PR if branch survived cleanup and no PR exists yet.
	if result.Branch != "" && detail.PRURL == "" && c.CreatePR != nil {
		if url, err := c.CreatePR(result.Branch); err == nil {
			detail.PRURL = url
			detail.BranchActions = c.computeBranchActions(detail)
		}
	}

	return result, nil
}
