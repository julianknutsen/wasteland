package sdk

import (
	"github.com/julianknutsen/wasteland/internal/commons"
)

// BrowseResult holds the items returned by Browse along with branch metadata.
type BrowseResult struct {
	Items     []commons.WantedSummary
	BranchIDs map[string]bool // wanted IDs that have a local mutation branch
}

// DetailResult holds the full picture of a wanted item for display.
type DetailResult struct {
	Item       *commons.WantedItem
	Completion *commons.CompletionRecord
	Stamp      *commons.Stamp
	Branch     string // mutation branch name ("" if none)
	MainStatus string // status on main ("" if no branch)
	PRURL      string // existing PR URL ("" if none)
	Delta      string // human-readable delta label ("" if none)
	Actions    []commons.Transition
	// BranchActions are mode-aware branch operations: "submit_pr", "apply", "discard".
	// Computed by the SDK based on mode, branch state, delta, and existing PR.
	BranchActions []string
}

// Browse queries the wanted board with filters, applying branch overlays in PR mode.
func (c *Client) Browse(filter commons.BrowseFilter) (*BrowseResult, error) {
	items, branchIDs, err := commons.BrowseWantedBranchAware(c.db, c.mode, c.rigHandle, filter)
	if err != nil {
		return nil, err
	}
	return &BrowseResult{Items: items, BranchIDs: branchIDs}, nil
}

// Detail fetches the complete state of a wanted item including actions.
func (c *Client) Detail(wantedID string) (*DetailResult, error) {
	if c.mode == "pr" {
		return c.detailPR(wantedID)
	}
	return c.detailWildWest(wantedID)
}

func (c *Client) detailPR(wantedID string) (*DetailResult, error) {
	state, err := commons.ResolveItemState(c.db, c.rigHandle, wantedID)
	if err != nil {
		return nil, err
	}
	effective := state.Effective()
	if effective == nil {
		// Fall back to main query if resolve found nothing.
		return c.detailWildWest(wantedID)
	}

	result := &DetailResult{
		Item:       effective,
		Completion: state.Completion,
		Stamp:      state.Stamp,
		Branch:     state.BranchName,
		Delta:      state.Delta(),
		Actions:    commons.AvailableTransitions(effective, c.rigHandle),
	}
	if state.Main != nil {
		result.MainStatus = state.Main.Status
	}
	if state.BranchName != "" && c.CheckPR != nil {
		result.PRURL = c.CheckPR(state.BranchName)
	}
	result.BranchActions = c.computeBranchActions(result)
	return result, nil
}

func (c *Client) detailWildWest(wantedID string) (*DetailResult, error) {
	item, completion, stamp, err := commons.QueryFullDetail(c.db, wantedID)
	if err != nil {
		return nil, err
	}
	return &DetailResult{
		Item:       item,
		Completion: completion,
		Stamp:      stamp,
		Actions:    commons.AvailableTransitions(item, c.rigHandle),
	}, nil
}

// computeBranchActions returns the mode-aware branch operations available for a detail result.
// This mirrors the TUI's actionHints() logic for delta resolution:
//   - PR mode with delta and no existing PR: ["submit_pr", "discard"]
//   - PR mode with delta and existing PR: ["discard"]
//   - Wild-west mode with delta: ["apply", "discard"]
//   - No branch or no delta: []
func (c *Client) computeBranchActions(r *DetailResult) []string {
	if r.Branch == "" || r.Delta == "" {
		return nil
	}
	var actions []string
	if c.mode == "pr" {
		if r.PRURL == "" {
			actions = append(actions, "submit_pr")
		}
	} else {
		actions = append(actions, "apply")
	}
	actions = append(actions, "discard")
	return actions
}

// Dashboard fetches the personal dashboard for the current rig handle.
func (c *Client) Dashboard() (*commons.DashboardData, error) {
	return commons.QueryMyDashboardBranchAware(c.db, c.mode, c.rigHandle)
}
