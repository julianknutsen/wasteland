package sdk

import (
	"sort"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
)

// --- test helpers ---

// actionNames converts transitions to sorted name strings.
func actionNames(actions []commons.Transition) []string {
	names := make([]string, len(actions))
	for i, a := range actions {
		names[i] = commons.TransitionName(a)
	}
	sort.Strings(names)
	return names
}

// assertActions checks that got transitions match want names (order-independent).
func assertActions(t *testing.T, got []commons.Transition, want []string) {
	t.Helper()
	gotNames := actionNames(got)
	wantSorted := make([]string, len(want))
	copy(wantSorted, want)
	sort.Strings(wantSorted)
	if len(gotNames) != len(wantSorted) {
		t.Fatalf("actions: got %v, want %v", gotNames, wantSorted)
	}
	for i := range gotNames {
		if gotNames[i] != wantSorted[i] {
			t.Fatalf("actions: got %v, want %v", gotNames, wantSorted)
		}
	}
}

// assertBranchActions checks that branch actions match exactly (order-sensitive).
func assertBranchActions(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("branch actions: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("branch actions: got %v, want %v", got, want)
		}
	}
}

// --- 1. Exhaustive state × role matrix ---

func TestAvailableActions(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		postedBy  string
		claimedBy string
		actor     string
		wantNames []string
	}{
		// --- open ---
		{"open/poster", "open", "alice", "", "alice", []string{"claim", "delete"}},
		{"open/other", "open", "alice", "", "bob", []string{"claim"}},

		// --- claimed ---
		{"claimed/poster", "claimed", "alice", "bob", "alice", []string{"unclaim"}},
		{"claimed/claimer", "claimed", "alice", "bob", "bob", []string{"unclaim", "done"}},
		{"claimed/other", "claimed", "alice", "bob", "carol", []string{}},

		// --- in_review ---
		{"in_review/poster", "in_review", "alice", "bob", "alice", []string{"accept", "reject", "close"}},
		{"in_review/claimer", "in_review", "alice", "bob", "bob", []string{}},
		{"in_review/other", "in_review", "alice", "bob", "carol", []string{}},
		{"in_review/poster=claimer", "in_review", "alice", "alice", "alice", []string{"reject", "close"}},

		// --- completed ---
		{"completed/poster", "completed", "alice", "bob", "alice", []string{}},
		{"completed/claimer", "completed", "alice", "bob", "bob", []string{}},
		{"completed/other", "completed", "alice", "bob", "carol", []string{}},

		// --- withdrawn ---
		{"withdrawn/poster", "withdrawn", "alice", "", "alice", []string{}},
		{"withdrawn/other", "withdrawn", "alice", "", "bob", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			db.seedItem(fakeItem{
				ID:          "w-1",
				Title:       "Test item",
				Status:      tt.status,
				PostedBy:    tt.postedBy,
				ClaimedBy:   tt.claimedBy,
				EffortLevel: "medium",
			})

			c := New(ClientConfig{DB: db, RigHandle: tt.actor, Mode: "wild-west"})
			result, err := c.Detail("w-1")
			if err != nil {
				t.Fatalf("Detail: %v", err)
			}
			assertActions(t, result.Actions, tt.wantNames)
		})
	}
}

// --- 2. Branch actions matrix ---

func TestBranchActionsMatrix(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		branch    string
		delta     string
		prURL     string
		hasDelete bool
		want      []string
	}{
		{"no_branch", "pr", "", "", "", false, nil},
		{"no_delta", "pr", "wl/bob/w-1", "", "", false, nil},
		{"pr/delta/no_pr", "pr", "wl/bob/w-1", "claim", "", false, []string{"submit_pr", "discard"}},
		{"pr/delta/with_pr", "pr", "wl/bob/w-1", "claim", "https://example.com/pr/1", false, []string{"discard"}},
		{"pr/delta/has_delete", "pr", "wl/bob/w-1", "new", "", true, []string{"submit_pr"}},
		{"wildwest/delta", "wild-west", "wl/bob/w-1", "claim", "", false, []string{"apply", "discard"}},
		{"wildwest/delta/has_delete", "wild-west", "wl/bob/w-1", "new", "", true, []string{"apply"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeBranchActions(tt.mode, tt.branch, tt.delta, tt.prURL, tt.hasDelete)
			assertBranchActions(t, got, tt.want)
		})
	}
}

// --- 3. Full lifecycle: wild-west mode ---

func TestLifecycle_WildWest(t *testing.T) {
	db := newFakeDB()
	db.seedItem(fakeItem{
		ID: "w-1", Title: "Fix bug", Status: "open",
		PostedBy: "alice", EffortLevel: "medium",
	})

	alice := New(ClientConfig{DB: db, RigHandle: "alice", Mode: "wild-west"})
	bob := New(ClientConfig{DB: db, RigHandle: "bob", Mode: "wild-west"})

	// 1. Verify open state: poster sees claim+delete, other sees claim.
	d, err := alice.Detail("w-1")
	if err != nil {
		t.Fatalf("Detail (alice/open): %v", err)
	}
	assertActions(t, d.Actions, []string{"claim", "delete"})

	d, err = bob.Detail("w-1")
	if err != nil {
		t.Fatalf("Detail (bob/open): %v", err)
	}
	assertActions(t, d.Actions, []string{"claim"})

	// 2. Bob claims → claimed.
	res, err := bob.Claim("w-1")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if res.Detail.Item.Status != "claimed" {
		t.Fatalf("expected claimed, got %s", res.Detail.Item.Status)
	}
	assertActions(t, res.Detail.Actions, []string{"unclaim", "done"})

	// Poster sees unclaim only.
	d, err = alice.Detail("w-1")
	if err != nil {
		t.Fatalf("Detail (alice/claimed): %v", err)
	}
	assertActions(t, d.Actions, []string{"unclaim"})

	// 3. Bob submits Done → in_review.
	res, err = bob.Done("w-1", "http://example.com/evidence")
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if res.Detail.Item.Status != "in_review" {
		t.Fatalf("expected in_review, got %s", res.Detail.Item.Status)
	}
	assertActions(t, res.Detail.Actions, []string{}) // claimer sees nothing

	// Poster sees accept/reject/close.
	d, err = alice.Detail("w-1")
	if err != nil {
		t.Fatalf("Detail (alice/in_review): %v", err)
	}
	assertActions(t, d.Actions, []string{"accept", "reject", "close"})

	// 4. Alice accepts → completed.
	res, err = alice.Accept("w-1", AcceptInput{Quality: 5, Reliability: 5, Severity: "minor"})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if res.Detail.Item.Status != "completed" {
		t.Fatalf("expected completed, got %s", res.Detail.Item.Status)
	}
	assertActions(t, res.Detail.Actions, []string{})
}

// --- 4. Full lifecycle: PR mode ---

func TestLifecycle_PR(t *testing.T) {
	db := newFakeDB()
	db.seedItem(fakeItem{
		ID: "w-1", Title: "Fix bug", Status: "open",
		PostedBy: "alice", EffortLevel: "medium",
	})

	bob := New(ClientConfig{DB: db, RigHandle: "bob", Mode: "pr"})

	// 1. Bob claims → branch created with delta.
	res, err := bob.Claim("w-1")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if res.Detail.Item.Status != "claimed" {
		t.Fatalf("expected claimed, got %s", res.Detail.Item.Status)
	}
	if res.Detail.Branch != "wl/bob/w-1" {
		t.Fatalf("expected branch wl/bob/w-1, got %q", res.Detail.Branch)
	}
	if res.Detail.Delta != "claim" {
		t.Fatalf("expected delta 'claim', got %q", res.Detail.Delta)
	}
	if res.Detail.MainStatus != "open" {
		t.Fatalf("expected main status 'open', got %q", res.Detail.MainStatus)
	}
	assertBranchActions(t, res.Detail.BranchActions, []string{"submit_pr", "discard"})

	// 2. Bob submits Done → branch updated, multi-hop delta.
	res, err = bob.Done("w-1", "http://example.com/evidence")
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if res.Detail.Item.Status != "in_review" {
		t.Fatalf("expected in_review, got %s", res.Detail.Item.Status)
	}
	if res.Detail.Branch != "wl/bob/w-1" {
		t.Fatalf("expected branch persists, got %q", res.Detail.Branch)
	}
	if res.Detail.Delta != "changes" {
		t.Fatalf("expected delta 'changes' (multi-hop), got %q", res.Detail.Delta)
	}

	// 3. Apply bob's branch → main is in_review.
	if err := bob.ApplyBranch("wl/bob/w-1"); err != nil {
		t.Fatalf("ApplyBranch (bob): %v", err)
	}
	if db.items["w-1"].Status != "in_review" {
		t.Fatalf("expected main in_review, got %s", db.items["w-1"].Status)
	}

	// 4. Alice accepts in PR mode → new branch.
	alice := New(ClientConfig{DB: db, RigHandle: "alice", Mode: "pr"})
	res, err = alice.Accept("w-1", AcceptInput{Quality: 5, Reliability: 5})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if res.Detail.Item.Status != "completed" {
		t.Fatalf("expected completed, got %s", res.Detail.Item.Status)
	}
	if res.Detail.Branch != "wl/alice/w-1" {
		t.Fatalf("expected branch wl/alice/w-1, got %q", res.Detail.Branch)
	}
	if res.Detail.Delta != "accept" {
		t.Fatalf("expected delta 'accept', got %q", res.Detail.Delta)
	}

	// 5. Apply alice's branch → main is completed.
	if err := alice.ApplyBranch("wl/alice/w-1"); err != nil {
		t.Fatalf("ApplyBranch (alice): %v", err)
	}
	if db.items["w-1"].Status != "completed" {
		t.Fatalf("expected main completed, got %s", db.items["w-1"].Status)
	}
}

// --- 5. Reject and retry cycle ---

func TestLifecycle_RejectAndRetry(t *testing.T) {
	db := newFakeDB()
	db.seedItem(fakeItem{
		ID: "w-1", Title: "Fix bug", Status: "open",
		PostedBy: "alice", EffortLevel: "medium",
	})

	alice := New(ClientConfig{DB: db, RigHandle: "alice", Mode: "wild-west"})
	bob := New(ClientConfig{DB: db, RigHandle: "bob", Mode: "wild-west"})

	// Claim → Done → Reject.
	if _, err := bob.Claim("w-1"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if _, err := bob.Done("w-1", "http://example.com/v1"); err != nil {
		t.Fatalf("Done: %v", err)
	}

	res, err := alice.Reject("w-1", "needs more work")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if res.Detail.Item.Status != "claimed" {
		t.Fatalf("expected claimed after reject, got %s", res.Detail.Item.Status)
	}
	// Poster sees unclaim.
	assertActions(t, res.Detail.Actions, []string{"unclaim"})

	// Claimer can still submit done.
	d, err := bob.Detail("w-1")
	if err != nil {
		t.Fatalf("Detail (bob/claimed): %v", err)
	}
	assertActions(t, d.Actions, []string{"unclaim", "done"})

	// Done again → Accept → completed.
	if _, err := bob.Done("w-1", "http://example.com/v2"); err != nil {
		t.Fatalf("Done v2: %v", err)
	}

	res, err = alice.Accept("w-1", AcceptInput{Quality: 5, Reliability: 5})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if res.Detail.Item.Status != "completed" {
		t.Fatalf("expected completed, got %s", res.Detail.Item.Status)
	}
	assertActions(t, res.Detail.Actions, []string{})
}

// --- 6. Mutation errors ---

func TestMutationErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		postedBy  string
		claimedBy string
		actor     string
		mutate    func(*Client, string) error
	}{
		{
			name:     "accept/no_completion_open",
			status:   "open",
			postedBy: "alice",
			actor:    "alice",
			mutate: func(c *Client, id string) error {
				_, err := c.Accept(id, AcceptInput{Quality: 5})
				return err
			},
		},
		{
			name:      "accept/no_completion_claimed",
			status:    "claimed",
			postedBy:  "alice",
			claimedBy: "bob",
			actor:     "alice",
			mutate: func(c *Client, id string) error {
				_, err := c.Accept(id, AcceptInput{Quality: 5})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			db.seedItem(fakeItem{
				ID: "w-1", Title: "Test", Status: tt.status,
				PostedBy: tt.postedBy, ClaimedBy: tt.claimedBy,
				EffortLevel: "medium",
			})

			c := New(ClientConfig{DB: db, RigHandle: tt.actor, Mode: "wild-west"})
			err := tt.mutate(c, "w-1")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// --- 7. PR auto-cleanup on revert ---

func TestAutoCleanup_PR(t *testing.T) {
	db := newFakeDB()
	db.seedItem(fakeItem{
		ID: "w-1", Title: "Fix bug", Status: "open",
		PostedBy: "alice", EffortLevel: "medium",
	})

	bob := New(ClientConfig{DB: db, RigHandle: "bob", Mode: "pr"})

	// Claim → branch created.
	res, err := bob.Claim("w-1")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if res.Detail.Branch == "" {
		t.Fatal("expected branch after claim")
	}

	// Unclaim → reverts to open (matches main) → auto-cleanup.
	res, err = bob.Unclaim("w-1")
	if err != nil {
		t.Fatalf("Unclaim: %v", err)
	}
	if res.Detail.Item.Status != "open" {
		t.Fatalf("expected open, got %s", res.Detail.Item.Status)
	}
	if res.Detail.Branch != "" {
		t.Fatalf("expected branch cleaned up, got %q", res.Detail.Branch)
	}
	if res.Detail.Delta != "" {
		t.Fatalf("expected no delta, got %q", res.Detail.Delta)
	}
	if len(res.Detail.BranchActions) != 0 {
		t.Fatalf("expected no branch actions, got %v", res.Detail.BranchActions)
	}
	if res.Hint != "reverted — branch cleaned up" {
		t.Fatalf("expected revert hint, got %q", res.Hint)
	}
}

// --- 8. Branch-only item deletion in PR mode ---

func TestDelete_BranchOnly_PR(t *testing.T) {
	db := newFakeDB()
	// Item only exists on branch, not on main.
	db.branches["wl/alice/w-1"] = true
	db.branchItems["wl/alice/w-1"] = map[string]*fakeItem{
		"w-1": {ID: "w-1", Title: "New thing", Status: "open", PostedBy: "alice", EffortLevel: "medium"},
	}

	c := New(ClientConfig{DB: db, RigHandle: "alice", Mode: "pr"})

	res, err := c.Delete("w-1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Branch should be cleaned up.
	if db.branches["wl/alice/w-1"] {
		t.Error("expected branch deleted")
	}
	// Hint should indicate cleanup.
	if res.Hint == "" {
		t.Error("expected hint about branch cleanup")
	}
	// Detail is nil for branch-only delete (no item left).
	if res.Detail != nil {
		t.Error("expected nil detail for branch-only delete")
	}
}
