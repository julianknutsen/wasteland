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
	// Preflight: verify this backend supports wild-west (direct upstream push).
	// RemoteDB fails here because the DoltHub API can't push fork→upstream.
	if err := c.db.CanWildWest(); err != nil {
		return nil, err
	}
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

	result := c.mutatePRResult(wantedID, branch, mainStatus)

	// Push the branch.
	var pushLog bytes.Buffer
	if err := c.db.PushBranch(branch, &pushLog); err != nil {
		if msg := strings.TrimSpace(pushLog.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("push branch: %w", err)
	}

	// Auto-cleanup: if mutation reverted item to main status, delete the branch.
	if mainStatus != "" && result.Detail.Item != nil && result.Detail.Item.Status == mainStatus {
		c.cleanupBranch(branch)
		result.Detail.Branch = ""
		result.Detail.BranchURL = ""
		result.Detail.MainStatus = ""
		result.Detail.Delta = ""
		result.Detail.PRURL = ""
		result.Detail.BranchActions = nil
		result.Branch = ""
		result.Hint = "reverted — branch cleaned up"
	}

	// Auto-submit PR if branch survived cleanup and no PR exists yet.
	if result.Branch != "" && result.Detail.PRURL == "" && c.CreatePR != nil {
		if url, err := c.CreatePR(result.Branch); err == nil {
			result.Detail.PRURL = url
			result.Detail.BranchActions = c.computeBranchActions(result.Detail)
		} else {
			result.Hint = fmt.Sprintf("PR creation failed: %v", err)
		}
	}

	return result, nil
}

// prIdempotent checks whether the branch already has the target status.
// If so, returns the current branch state without creating another commit.
// This prevents duplicate commits when the DoltHub write API
// (write/main/{branch}) replays from main on every call.
// Returns nil when the mutation should proceed normally.
func (c *Client) prIdempotent(wantedID, targetStatus string) *MutationResult {
	if c.mode != "pr" {
		return nil
	}
	branch := commons.BranchName(c.rigHandle, wantedID)
	branchStatus, _, _ := commons.QueryItemStatus(c.db, wantedID, branch)
	if branchStatus != targetStatus {
		return nil
	}
	mainStatus, _, _ := commons.QueryItemStatus(c.db, wantedID, "main")
	if branchStatus == mainStatus {
		// Branch matches main — mutation hasn't been applied yet.
		return nil
	}
	return c.mutatePRResult(wantedID, branch, mainStatus)
}

// mutatePRResult reads the current branch state and builds a MutationResult.
// Used after Exec and also for the idempotency early-return path.
func (c *Client) mutatePRResult(wantedID, branch, mainStatus string) *MutationResult {
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
		detail.Delta = commons.ComputeDelta(mainStatus, item.Status, true)
	}
	if branch != "" && c.CheckPR != nil {
		detail.PRURL = c.CheckPR(branch)
	}
	if branch != "" && c.BranchURL != nil {
		detail.BranchURL = c.BranchURL(branch)
	}
	detail.BranchActions = c.computeBranchActions(detail)

	return &MutationResult{Detail: detail, Branch: branch}
}
