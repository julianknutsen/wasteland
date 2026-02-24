package commons

import (
	"fmt"
	"io"
)

// Transition represents a lifecycle state change for a wanted item.
type Transition int

// Lifecycle transitions for wanted items.
const (
	TransitionClaim   Transition = iota // open → claimed
	TransitionUnclaim                   // claimed → open
	TransitionDone                      // claimed → in_review
	TransitionAccept                    // in_review → completed
	TransitionReject                    // in_review → claimed
	TransitionClose                     // in_review → completed
	TransitionDelete                    // open → withdrawn
	TransitionUpdate                    // open → open
)

// transitionRule defines the required from-status and resulting to-status.
type transitionRule struct {
	from string
	to   string
	name string
}

var transitionRules = map[Transition]transitionRule{
	TransitionClaim:   {from: "open", to: "claimed", name: "claim"},
	TransitionUnclaim: {from: "claimed", to: "open", name: "unclaim"},
	TransitionDone:    {from: "claimed", to: "in_review", name: "done"},
	TransitionAccept:  {from: "in_review", to: "completed", name: "accept"},
	TransitionReject:  {from: "in_review", to: "claimed", name: "reject"},
	TransitionClose:   {from: "in_review", to: "completed", name: "close"},
	TransitionDelete:  {from: "open", to: "withdrawn", name: "delete"},
	TransitionUpdate:  {from: "open", to: "open", name: "update"},
}

// ValidateTransition checks if a transition is valid from the given status.
// Returns the new status or an error with a clear message.
func ValidateTransition(currentStatus string, t Transition) (string, error) {
	rule, ok := transitionRules[t]
	if !ok {
		return "", fmt.Errorf("unknown transition %d", t)
	}
	if currentStatus != rule.from {
		return "", fmt.Errorf("cannot %s: item is %s, not %s", rule.name, currentStatus, rule.from)
	}
	return rule.to, nil
}

// ItemLocation describes where a wanted item's state currently lives.
type ItemLocation struct {
	LocalStatus     string // status in local working copy
	OriginStatus    string // status on origin/main ("" if not found)
	UpstreamStatus  string // status on upstream/main ("" if not found)
	FetchedOrigin   bool   // whether origin fetch succeeded
	FetchedUpstream bool   // whether upstream fetch succeeded
}

// DetectItemLocation fetches both remotes and queries item state at each ref.
func DetectItemLocation(dbDir, wantedID string) (*ItemLocation, error) {
	loc := &ItemLocation{}

	// Fetch remotes (best-effort — failures are recorded but not fatal).
	if err := FetchRemote(dbDir, "origin"); err == nil {
		loc.FetchedOrigin = true
	}
	if err := FetchRemote(dbDir, "upstream"); err == nil {
		loc.FetchedUpstream = true
	}

	// Query local status (working copy).
	loc.LocalStatus = QueryItemStatusAsOf(dbDir, wantedID, "")

	// Query remote statuses using AS OF.
	if loc.FetchedOrigin {
		loc.OriginStatus = QueryItemStatusAsOf(dbDir, wantedID, "origin/main")
	}
	if loc.FetchedUpstream {
		loc.UpstreamStatus = QueryItemStatusAsOf(dbDir, wantedID, "upstream/main")
	}

	return loc, nil
}

// PushTarget determines what needs to be pushed based on location + mode.
type PushTarget struct {
	PushOrigin   bool   // force push local main to origin
	PushUpstream bool   // push to upstream (wild-west only)
	Hint         string // user-facing hint ("create PR to upstream", etc.)
}

// ResolvePushTarget determines the minimum push operation needed given
// the workflow mode and the item's location across remotes.
func ResolvePushTarget(mode string, loc *ItemLocation) PushTarget {
	if mode != "pr" {
		// Wild-west: always push to both remotes (existing behavior).
		return PushTarget{PushOrigin: true, PushUpstream: true}
	}

	// PR mode: only push to remotes where state needs to change.
	localStatus := loc.LocalStatus

	// If local matches origin, nothing to push — state is already on the fork.
	if localStatus == loc.OriginStatus {
		hint := ""
		if localStatus != loc.UpstreamStatus {
			hint = "Origin is up to date. Create a PR to push changes upstream."
		}
		return PushTarget{Hint: hint}
	}

	// Local differs from origin: push to origin.
	hint := ""
	if loc.OriginStatus == loc.UpstreamStatus && localStatus != loc.UpstreamStatus {
		hint = "Pushed to origin. Create a PR when ready to push upstream."
	}
	return PushTarget{PushOrigin: true, Hint: hint}
}

// PushOriginMain force-pushes local main to origin.
// Used in PR mode when only the fork needs updating.
func PushOriginMain(dbDir string, stdout io.Writer) error {
	return PushBranchToRemoteForce(dbDir, "origin", "main", true, stdout)
}

// CanPerformTransition checks whether actor can perform transition t on item.
func CanPerformTransition(item *WantedItem, t Transition, actor string) bool {
	if item == nil {
		return false
	}
	switch t {
	case TransitionClaim:
		return true // any rig can claim
	case TransitionUnclaim:
		return item.ClaimedBy == actor || item.PostedBy == actor
	case TransitionDone:
		return item.ClaimedBy == actor
	case TransitionAccept:
		return item.PostedBy == actor && item.ClaimedBy != actor
	case TransitionReject:
		return item.PostedBy == actor
	case TransitionClose:
		return item.PostedBy == actor
	case TransitionDelete:
		return true // any rig can delete
	default:
		return false
	}
}

// TransitionLabel returns a human-readable in-progress label for a transition.
func TransitionLabel(t Transition) string {
	switch t {
	case TransitionClaim:
		return "Claiming..."
	case TransitionUnclaim:
		return "Unclaiming..."
	case TransitionReject:
		return "Rejecting..."
	case TransitionClose:
		return "Closing..."
	case TransitionDelete:
		return "Deleting..."
	default:
		return "Working..."
	}
}

// TransitionName returns the short name for a transition (e.g. "claim").
func TransitionName(t Transition) string {
	if rule, ok := transitionRules[t]; ok {
		return rule.name
	}
	return "unknown"
}

// TransitionRequiresInput returns a non-empty hint if a transition requires
// additional input that can't be gathered in the TUI (must use CLI).
func TransitionRequiresInput(t Transition) string {
	switch t {
	case TransitionDone:
		return "requires evidence URL"
	case TransitionAccept:
		return "requires quality rating"
	default:
		return ""
	}
}

// DeltaLabel returns a human-readable label for a state delta.
// Single-hop deltas map to transition names: "claim", "done", "reject".
// Multi-hop or unrecognized deltas return "changes".
func DeltaLabel(mainStatus, branchStatus string) string {
	for _, rule := range transitionRules {
		if rule.from == mainStatus && rule.to == branchStatus {
			return rule.name
		}
	}
	return "changes"
}

// AvailableTransitions returns transitions valid for item that actor can perform.
func AvailableTransitions(item *WantedItem, actor string) []Transition {
	if item == nil {
		return nil
	}
	all := []Transition{
		TransitionClaim,
		TransitionUnclaim,
		TransitionDone,
		TransitionAccept,
		TransitionReject,
		TransitionClose,
		TransitionDelete,
	}
	var result []Transition
	for _, t := range all {
		if _, err := ValidateTransition(item.Status, t); err == nil {
			if CanPerformTransition(item, t, actor) {
				result = append(result, t)
			}
		}
	}
	return result
}
