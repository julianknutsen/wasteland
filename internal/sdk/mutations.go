package sdk

import (
	"fmt"

	"github.com/julianknutsen/wasteland/internal/commons"
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
	stmts := []string{commons.ClaimWantedDML(wantedID, c.rigHandle)}
	return c.mutate(wantedID, "wl claim: "+wantedID, stmts...)
}

// Unclaim reverts a claimed wanted item to open.
func (c *Client) Unclaim(wantedID string) (*MutationResult, error) {
	stmts := []string{commons.UnclaimWantedDML(wantedID)}
	return c.mutate(wantedID, "wl unclaim: "+wantedID, stmts...)
}

// Done submits completion evidence for a claimed wanted item.
func (c *Client) Done(wantedID, evidence string) (*MutationResult, error) {
	completionID := commons.GeneratePrefixedID("c", wantedID, c.rigHandle)
	stmts := commons.SubmitCompletionDML(completionID, wantedID, c.rigHandle, evidence, c.hopURI)
	return c.mutate(wantedID, "wl done: "+wantedID, stmts...)
}

// Accept validates a completion, creates a stamp, and marks the item completed.
func (c *Client) Accept(wantedID string, input AcceptInput) (*MutationResult, error) {
	// Look up the completion to get its ID.
	completion, err := commons.QueryCompletion(c.db, wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying completion: %w", err)
	}

	stamp := &commons.Stamp{
		ID:          commons.GeneratePrefixedID("s", wantedID, c.rigHandle),
		Author:      c.rigHandle,
		Subject:     completion.ID,
		Quality:     input.Quality,
		Reliability: input.Reliability,
		Severity:    input.Severity,
		ContextID:   completion.ID,
		ContextType: "completion",
		SkillTags:   input.SkillTags,
		Message:     input.Message,
	}

	stmts := commons.AcceptCompletionDML(wantedID, completion.ID, c.rigHandle, c.hopURI, stamp)
	return c.mutate(wantedID, "wl accept: "+wantedID, stmts...)
}

// Reject rejects a completion, reverting the item from in_review to claimed.
func (c *Client) Reject(wantedID, reason string) (*MutationResult, error) {
	stmts := commons.RejectCompletionDML(wantedID)
	msg := "wl reject: " + wantedID
	if reason != "" {
		msg += " — " + reason
	}
	return c.mutate(wantedID, msg, stmts...)
}

// Close marks an in_review item as completed without a stamp.
func (c *Client) Close(wantedID string) (*MutationResult, error) {
	stmts := []string{commons.CloseWantedDML(wantedID)}
	return c.mutate(wantedID, "wl close: "+wantedID, stmts...)
}

// Delete soft-deletes a wanted item by setting status=withdrawn.
// In PR mode, if the item only exists on a branch (never on main),
// we skip the mutation and just clean up the branch instead.
func (c *Client) Delete(wantedID string) (*MutationResult, error) {
	if c.mode == "pr" {
		branch := commons.BranchName(c.rigHandle, wantedID)
		mainStatus, _, _ := commons.QueryItemStatus(c.db, wantedID, "main")
		if mainStatus == "" {
			// Item only exists on branch — clean up branch entirely.
			// Deleting the remote branch auto-closes any existing PR.
			c.mu.Lock()
			defer c.mu.Unlock()
			_ = c.db.DeleteBranch(branch)
			_ = c.db.DeleteRemoteBranch(branch)
			return &MutationResult{
				Hint: "branch-only item — branch deleted",
			}, nil
		}
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
