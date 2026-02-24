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
