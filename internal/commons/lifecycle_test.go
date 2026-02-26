package commons

import "testing"

func TestValidateTransition(t *testing.T) {
	valid := []struct {
		name       string
		from       string
		transition Transition
		wantTo     string
	}{
		{"claim from open", "open", TransitionClaim, "claimed"},
		{"unclaim from claimed", "claimed", TransitionUnclaim, "open"},
		{"done from claimed", "claimed", TransitionDone, "in_review"},
		{"accept from in_review", "in_review", TransitionAccept, "completed"},
		{"reject from in_review", "in_review", TransitionReject, "claimed"},
		{"close from in_review", "in_review", TransitionClose, "completed"},
		{"delete from open", "open", TransitionDelete, "withdrawn"},
		{"update from open", "open", TransitionUpdate, "open"},
	}

	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateTransition(tc.from, tc.transition)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantTo {
				t.Errorf("got %q, want %q", got, tc.wantTo)
			}
		})
	}

	invalid := []struct {
		name       string
		from       string
		transition Transition
	}{
		{"claim from claimed", "claimed", TransitionClaim},
		{"claim from in_review", "in_review", TransitionClaim},
		{"claim from completed", "completed", TransitionClaim},
		{"unclaim from open", "open", TransitionUnclaim},
		{"unclaim from in_review", "in_review", TransitionUnclaim},
		{"done from open", "open", TransitionDone},
		{"done from in_review", "in_review", TransitionDone},
		{"accept from open", "open", TransitionAccept},
		{"accept from claimed", "claimed", TransitionAccept},
		{"reject from open", "open", TransitionReject},
		{"reject from claimed", "claimed", TransitionReject},
		{"close from open", "open", TransitionClose},
		{"close from claimed", "claimed", TransitionClose},
		{"delete from claimed", "claimed", TransitionDelete},
		{"delete from in_review", "in_review", TransitionDelete},
		{"update from claimed", "claimed", TransitionUpdate},
		{"update from in_review", "in_review", TransitionUpdate},
	}

	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateTransition(tc.from, tc.transition)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCanPerformTransition(t *testing.T) {
	item := &WantedItem{
		ID:        "w-test",
		Status:    "claimed",
		PostedBy:  "poster",
		ClaimedBy: "claimer",
	}

	tests := []struct {
		name  string
		t     Transition
		actor string
		want  bool
	}{
		{"claim anyone", TransitionClaim, "random", true},
		{"unclaim by claimer", TransitionUnclaim, "claimer", true},
		{"unclaim by poster", TransitionUnclaim, "poster", true},
		{"unclaim by other", TransitionUnclaim, "other", false},
		{"done by claimer", TransitionDone, "claimer", true},
		{"done by other", TransitionDone, "other", false},
		{"delete by poster", TransitionDelete, "poster", true},
		{"delete by other", TransitionDelete, "random", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CanPerformTransition(item, tc.t, tc.actor)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	// nil item always returns false.
	if CanPerformTransition(nil, TransitionClaim, "x") {
		t.Error("nil item should return false")
	}
}

func TestCanPerformTransition_Accept(t *testing.T) {
	item := &WantedItem{
		ID:        "w-test",
		Status:    "in_review",
		PostedBy:  "poster",
		ClaimedBy: "claimer",
	}

	// Poster (non-claimer) can accept.
	if !CanPerformTransition(item, TransitionAccept, "poster") {
		t.Error("poster should be able to accept")
	}
	// Claimer cannot accept own work.
	if CanPerformTransition(item, TransitionAccept, "claimer") {
		t.Error("claimer should not be able to accept own work")
	}
	// Random cannot accept.
	if CanPerformTransition(item, TransitionAccept, "random") {
		t.Error("random should not be able to accept")
	}
}

func TestTransitionLabel(t *testing.T) {
	tests := []struct {
		t    Transition
		want string
	}{
		{TransitionClaim, "Claiming..."},
		{TransitionUnclaim, "Unclaiming..."},
		{TransitionReject, "Rejecting..."},
		{TransitionClose, "Closing..."},
		{TransitionDelete, "Deleting..."},
	}
	for _, tc := range tests {
		if got := TransitionLabel(tc.t); got != tc.want {
			t.Errorf("TransitionLabel(%v) = %q, want %q", tc.t, got, tc.want)
		}
	}
	// Unknown transition returns "Working..."
	if got := TransitionLabel(Transition(99)); got != "Working..." {
		t.Errorf("unknown transition label = %q, want %q", got, "Working...")
	}
}

func TestTransitionName(t *testing.T) {
	if got := TransitionName(TransitionClaim); got != "claim" {
		t.Errorf("got %q, want %q", got, "claim")
	}
	if got := TransitionName(Transition(99)); got != "unknown" {
		t.Errorf("got %q, want %q", got, "unknown")
	}
}

func TestTransitionRequiresInput(t *testing.T) {
	if got := TransitionRequiresInput(TransitionDone); got == "" {
		t.Error("done should require input")
	}
	if got := TransitionRequiresInput(TransitionAccept); got == "" {
		t.Error("accept should require input")
	}
	if got := TransitionRequiresInput(TransitionClaim); got != "" {
		t.Errorf("claim should not require input, got %q", got)
	}
}

func TestAvailableTransitions(t *testing.T) {
	// Open item, poster is "poster", actor is "random".
	item := &WantedItem{
		ID:       "w-test",
		Status:   "open",
		PostedBy: "poster",
	}

	// Random can only claim (not delete — only poster can delete).
	got := AvailableTransitions(item, "random")
	if len(got) != 1 {
		t.Fatalf("expected 1 transition, got %d: %v", len(got), got)
	}
	if got[0] != TransitionClaim {
		t.Errorf("expected [claim], got %v", got)
	}

	// Poster gets claim + delete from open.
	got = AvailableTransitions(item, "poster")
	if len(got) != 2 {
		t.Fatalf("expected 2 transitions for poster on open, got %d: %v", len(got), got)
	}
	if got[0] != TransitionClaim || got[1] != TransitionDelete {
		t.Errorf("expected [claim, delete], got %v", got)
	}

	// nil item returns nil.
	if AvailableTransitions(nil, "x") != nil {
		t.Error("nil item should return nil")
	}
}

func TestComputeDelta(t *testing.T) {
	tests := []struct {
		name         string
		mainStatus   string
		branchStatus string
		branchExists bool
		want         string
	}{
		{"no branch", "open", "claimed", false, ""},
		{"new item", "", "open", true, "new"},
		{"claim delta", "open", "claimed", true, "claim"},
		{"done delta", "claimed", "in_review", true, "done"},
		{"same status", "open", "open", true, "changes"},
		{"multi-hop", "open", "completed", true, "changes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeDelta(tc.mainStatus, tc.branchStatus, tc.branchExists)
			if got != tc.want {
				t.Errorf("ComputeDelta(%q, %q, %v) = %q, want %q", tc.mainStatus, tc.branchStatus, tc.branchExists, got, tc.want)
			}
		})
	}
}

func TestDeltaLabel(t *testing.T) {
	tests := []struct {
		name         string
		mainStatus   string
		branchStatus string
		want         string
	}{
		{"claim", "open", "claimed", "claim"},
		{"unclaim", "claimed", "open", "unclaim"},
		{"done", "claimed", "in_review", "done"},
		{"accept", "in_review", "completed", "accept"},
		{"reject", "in_review", "claimed", "reject"},
		{"delete", "open", "withdrawn", "delete"},
		{"multi-hop open to in_review", "open", "in_review", "changes"},
		{"multi-hop open to completed", "open", "completed", "changes"},
		{"same status", "open", "open", "update"},
		{"unrecognized", "completed", "open", "changes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeltaLabel(tc.mainStatus, tc.branchStatus)
			if got != tc.want {
				t.Errorf("DeltaLabel(%q, %q) = %q, want %q", tc.mainStatus, tc.branchStatus, got, tc.want)
			}
		})
	}
}

func TestItemState_Effective_BranchOverMain(t *testing.T) {
	s := &ItemState{
		Main:   &WantedItem{ID: "w-1", Status: "open"},
		Branch: &WantedItem{ID: "w-1", Status: "claimed"},
	}
	if e := s.Effective(); e.Status != "claimed" {
		t.Errorf("Effective() = %q, want branch status %q", e.Status, "claimed")
	}
}

func TestItemState_Effective_MainOnly(t *testing.T) {
	s := &ItemState{
		Main: &WantedItem{ID: "w-1", Status: "open"},
	}
	if e := s.Effective(); e.Status != "open" {
		t.Errorf("Effective() = %q, want main status %q", e.Status, "open")
	}
}

func TestItemState_Effective_Nil(t *testing.T) {
	s := &ItemState{}
	if s.Effective() != nil {
		t.Error("Effective() should be nil when both Main and Branch are nil")
	}
}

func TestItemState_EffectiveStatus(t *testing.T) {
	s := &ItemState{Main: &WantedItem{Status: "open"}}
	if got := s.EffectiveStatus(); got != "open" {
		t.Errorf("EffectiveStatus() = %q, want %q", got, "open")
	}
	empty := &ItemState{}
	if got := empty.EffectiveStatus(); got != "" {
		t.Errorf("EffectiveStatus() on empty = %q, want %q", got, "")
	}
}

func TestItemState_Delta(t *testing.T) {
	tests := []struct {
		name string
		s    ItemState
		want string
	}{
		{"no branch", ItemState{Main: &WantedItem{Status: "open"}}, ""},
		{"no main (new)", ItemState{Branch: &WantedItem{Status: "claimed"}}, "new"},
		{"same status (changes)", ItemState{
			Main:   &WantedItem{Status: "open"},
			Branch: &WantedItem{Status: "open"},
		}, "changes"},
		{"claim delta", ItemState{
			Main:   &WantedItem{Status: "open"},
			Branch: &WantedItem{Status: "claimed"},
		}, "claim"},
		{"done delta", ItemState{
			Main:   &WantedItem{Status: "claimed"},
			Branch: &WantedItem{Status: "in_review"},
		}, "done"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Delta(); got != tc.want {
				t.Errorf("Delta() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolvePushTarget_WildWest(t *testing.T) {
	loc := &ItemLocation{
		LocalStatus:    "claimed",
		OriginStatus:   "open",
		UpstreamStatus: "open",
	}
	pt := ResolvePushTarget("wild-west", loc)
	if !pt.PushOrigin || !pt.PushUpstream {
		t.Errorf("wild-west should push to both: got origin=%v upstream=%v", pt.PushOrigin, pt.PushUpstream)
	}
}

func TestResolvePushTarget_PR_LocalDiffersFromOrigin(t *testing.T) {
	loc := &ItemLocation{
		LocalStatus:    "claimed",
		OriginStatus:   "open",
		UpstreamStatus: "open",
	}
	pt := ResolvePushTarget("pr", loc)
	if !pt.PushOrigin {
		t.Error("PR mode with local!=origin should push to origin")
	}
	if pt.PushUpstream {
		t.Error("PR mode should never push to upstream")
	}
	if pt.Hint == "" {
		t.Error("expected hint about creating PR")
	}
}

func TestResolvePushTarget_PR_LocalMatchesOrigin_DiffersUpstream(t *testing.T) {
	loc := &ItemLocation{
		LocalStatus:    "claimed",
		OriginStatus:   "claimed",
		UpstreamStatus: "open",
	}
	pt := ResolvePushTarget("pr", loc)
	if pt.PushOrigin {
		t.Error("should not push to origin when local matches origin")
	}
	if pt.PushUpstream {
		t.Error("PR mode should never push to upstream")
	}
	if pt.Hint == "" {
		t.Error("expected hint about creating PR to upstream")
	}
}

func TestResolvePushTarget_PR_AllMatch(t *testing.T) {
	loc := &ItemLocation{
		LocalStatus:    "claimed",
		OriginStatus:   "claimed",
		UpstreamStatus: "claimed",
	}
	pt := ResolvePushTarget("pr", loc)
	if pt.PushOrigin || pt.PushUpstream {
		t.Error("should not push when all match")
	}
	if pt.Hint != "" {
		t.Errorf("expected no hint when fully synced, got %q", pt.Hint)
	}
}

func TestResolvePushTarget_PR_OriginDiffersUpstream(t *testing.T) {
	// Local differs from origin, and origin already differs from upstream.
	// Still push to origin — the PR handles upstream.
	loc := &ItemLocation{
		LocalStatus:    "in_review",
		OriginStatus:   "claimed",
		UpstreamStatus: "open",
	}
	pt := ResolvePushTarget("pr", loc)
	if !pt.PushOrigin {
		t.Error("should push to origin when local differs from origin")
	}
	if pt.PushUpstream {
		t.Error("PR mode should never push to upstream")
	}
}

func TestResolvePushTarget_PR_UndoOnFork(t *testing.T) {
	// Unclaimed locally, origin still claimed, upstream still open.
	// Push to origin to sync the undo — no upstream noise.
	loc := &ItemLocation{
		LocalStatus:    "open",
		OriginStatus:   "claimed",
		UpstreamStatus: "open",
	}
	pt := ResolvePushTarget("pr", loc)
	if !pt.PushOrigin {
		t.Error("should push undo to origin")
	}
	if pt.PushUpstream {
		t.Error("PR mode should never push to upstream")
	}
}
