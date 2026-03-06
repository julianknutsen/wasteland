package sdk

import (
	"fmt"

	"github.com/gastownhall/wasteland/internal/commons"
)

// AcceptInput holds the parameters for accepting a completion.
type AcceptInput struct {
	Quality     int
	Reliability int
	Severity    string
	SkillTags   []string
	Message     string
}

// PostInput holds the parameters for posting a new wanted item.
type PostInput struct {
	Title       string
	Description string
	Project     string
	Type        string
	Priority    int
	EffortLevel string
	Tags        []string
}

// Claim claims a wanted item for the current rig.
func (c *Client) Claim(wantedID string) (*MutationResult, error) {
	if result := c.prIdempotent(wantedID, "claimed"); result != nil {
		return result, nil
	}
	stmts := []string{commons.ClaimWantedDML(wantedID, c.rigHandle)}
	return c.mutate(wantedID, "wl claim: "+wantedID, stmts...)
}

// Unclaim reverts a claimed wanted item to open.
func (c *Client) Unclaim(wantedID string) (*MutationResult, error) {
	if result := c.prIdempotent(wantedID, "open"); result != nil {
		return result, nil
	}
	stmts := []string{commons.UnclaimWantedDML(wantedID)}
	return c.mutate(wantedID, "wl unclaim: "+wantedID, stmts...)
}

// Done submits completion evidence for a claimed wanted item.
func (c *Client) Done(wantedID, evidence string) (*MutationResult, error) {
	if result := c.prIdempotent(wantedID, "in_review"); result != nil {
		return result, nil
	}
	completionID := commons.GeneratePrefixedID("c", wantedID, c.rigHandle)
	stmts := commons.SubmitCompletionDML(completionID, wantedID, c.rigHandle, evidence, c.hopURI)
	return c.mutate(wantedID, "wl done: "+wantedID, stmts...)
}

// Accept validates a completion, creates a stamp, and marks the item completed.
func (c *Client) Accept(wantedID string, input AcceptInput) (*MutationResult, error) {
	// Hold the mutex for the entire operation to prevent concurrent Accept()
	// calls from both passing the idempotent check on the same completion.
	c.mu.Lock()
	defer c.mu.Unlock()

	if result := c.prIdempotentLocked(wantedID, "completed"); result != nil {
		return result, nil
	}

	// Look up the completion to get its ID and worker handle.
	completion, err := commons.QueryCompletion(c.db, wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying completion: %w", err)
	}
	if completion == nil {
		return nil, fmt.Errorf("no completion found for item %s", wantedID)
	}

	// Self-accept guard: the accepting rig must not be the one who completed the work.
	if completion.CompletedBy == c.rigHandle {
		return nil, fmt.Errorf("cannot accept your own completion")
	}

	stamp := &commons.Stamp{
		ID:          commons.GeneratePrefixedID("s", wantedID, c.rigHandle),
		Author:      c.rigHandle,
		Subject:     completion.CompletedBy,
		Quality:     input.Quality,
		Reliability: input.Reliability,
		Severity:    input.Severity,
		ContextID:   completion.ID,
		ContextType: "completion",
		SkillTags:   input.SkillTags,
		Message:     input.Message,
	}

	stmts := commons.AcceptCompletionDML(wantedID, completion.ID, c.rigHandle, c.hopURI, stamp)
	return c.mutateLocked(wantedID, "wl accept: "+wantedID, stmts...)
}

// AcceptUpstream adopts a fork submission, creating a completion and stamp on the poster's branch.
func (c *Client) AcceptUpstream(wantedID, submitterHandle string, input AcceptInput) (*MutationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if result := c.prIdempotentLocked(wantedID, "completed"); result != nil {
		return result, nil
	}

	if c.ListPendingItems == nil {
		return nil, fmt.Errorf("upstream PR listing not available")
	}

	pending, err := c.ListPendingItems()
	if err != nil {
		return nil, fmt.Errorf("listing pending items: %w", err)
	}

	items := pending[wantedID]
	var match *PendingItem
	for i := range items {
		if items[i].RigHandle == submitterHandle {
			match = &items[i]
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no pending submission from %s", submitterHandle)
	}

	if match.Status != "in_review" {
		return nil, fmt.Errorf("submission is not in review")
	}
	if match.CompletedBy == "" {
		return nil, fmt.Errorf("submission has no completion data")
	}

	if submitterHandle == c.rigHandle {
		return nil, fmt.Errorf("cannot accept your own completion")
	}

	completionID := commons.GeneratePrefixedID("c", wantedID, match.CompletedBy)
	stamp := &commons.Stamp{
		ID:          commons.GeneratePrefixedID("s", wantedID, c.rigHandle),
		Author:      c.rigHandle,
		Subject:     match.CompletedBy,
		Quality:     input.Quality,
		Reliability: input.Reliability,
		Severity:    input.Severity,
		ContextID:   completionID,
		ContextType: "completion",
		SkillTags:   input.SkillTags,
		Message:     input.Message,
	}

	stmts := commons.AcceptUpstreamDML(wantedID, completionID, match.CompletedBy, match.Evidence, c.rigHandle, c.hopURI, stamp)
	return c.mutateLocked(wantedID, "wl accept-upstream: "+wantedID, stmts...)
}

// Reject rejects a completion, reverting the item from in_review to claimed.
func (c *Client) Reject(wantedID, reason string) (*MutationResult, error) {
	if result := c.prIdempotent(wantedID, "claimed"); result != nil {
		return result, nil
	}
	stmts := commons.RejectCompletionDML(wantedID)
	msg := "wl reject: " + wantedID
	if reason != "" {
		if len(reason) > 500 {
			reason = reason[:500] + "..."
		}
		msg += " — " + reason
	}
	return c.mutate(wantedID, msg, stmts...)
}

// Close marks an in_review item as completed without a stamp.
func (c *Client) Close(wantedID string) (*MutationResult, error) {
	if result := c.prIdempotent(wantedID, "completed"); result != nil {
		return result, nil
	}
	stmts := []string{commons.CloseWantedDML(wantedID)}
	return c.mutate(wantedID, "wl close: "+wantedID, stmts...)
}

// Delete soft-deletes a wanted item by setting status=withdrawn.
// In PR mode, if the item only exists on a branch (never on main),
// we skip the mutation and just clean up the branch instead.
func (c *Client) Delete(wantedID string) (*MutationResult, error) {
	if c.mode == "pr" {
		// Hold lock for the entire check-then-act to prevent a concurrent
		// Post from creating the item on main between the query and cleanup.
		c.mu.Lock()

		if result := c.prIdempotentLocked(wantedID, "withdrawn"); result != nil {
			c.mu.Unlock()
			return result, nil
		}

		branch := commons.BranchName(c.rigHandle, wantedID)
		mainStatus, _, _ := commons.QueryItemStatus(c.db, wantedID, "main")
		if mainStatus == "" {
			// Item only exists on branch — clean up branch and close any PR.
			c.cleanupBranch(branch)
			c.mu.Unlock()
			return &MutationResult{
				Branch: branch,
				Hint:   "branch-only item — branch deleted",
			}, nil
		}
		c.mu.Unlock()
	}
	stmts := []string{commons.DeleteWantedDML(wantedID)}
	return c.mutate(wantedID, "wl delete: "+wantedID, stmts...)
}

// Post creates a new wanted item.
func (c *Client) Post(input PostInput) (*MutationResult, error) {
	id := commons.GenerateWantedID(input.Title)
	item := &commons.WantedItem{
		ID:          id,
		Title:       input.Title,
		Description: input.Description,
		Project:     input.Project,
		Type:        input.Type,
		Priority:    input.Priority,
		EffortLevel: input.EffortLevel,
		Tags:        input.Tags,
		PostedBy:    c.rigHandle,
	}

	dml, err := commons.InsertWantedDML(item)
	if err != nil {
		return nil, err
	}
	return c.mutate(id, "wl post: "+id, dml)
}

// Update modifies mutable fields on an open wanted item.
func (c *Client) Update(wantedID string, fields *commons.WantedUpdate) (*MutationResult, error) {
	dml, err := commons.UpdateWantedDML(wantedID, fields)
	if err != nil {
		return nil, err
	}
	return c.mutate(wantedID, "wl update: "+wantedID, dml)
}
