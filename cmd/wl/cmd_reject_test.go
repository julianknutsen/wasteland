package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/commons"
)

func TestRejectCompletion_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "poster-rig"})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "https://github.com/pr/1")

	err := rejectCompletion(store, "w-abc", "poster-rig", "tests failing")
	if err != nil {
		t.Fatalf("rejectCompletion() error: %v", err)
	}

	item, _ := store.QueryWanted("w-abc")
	if item.Status != "claimed" {
		t.Errorf("Status = %q, want %q", item.Status, "claimed")
	}

	// Completion should be deleted.
	_, err = store.QueryCompletion("w-abc")
	if err == nil {
		t.Error("expected completion to be deleted after reject")
	}
}

func TestRejectCompletion_NotInReview(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "poster-rig"})

	err := rejectCompletion(store, "w-abc", "poster-rig", "")
	if err == nil {
		t.Fatal("rejectCompletion() expected error for non-in_review item")
	}
	if !strings.Contains(err.Error(), "not in_review") {
		t.Errorf("error = %q, want to contain 'not in_review'", err.Error())
	}
}

func TestRejectCompletion_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	err := rejectCompletion(store, "w-nonexistent", "poster-rig", "")
	if err == nil {
		t.Fatal("rejectCompletion() expected error for missing item")
	}
}

func TestRejectCompletion_NotPoster(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "poster-rig"})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "evidence")

	err := rejectCompletion(store, "w-abc", "other-rig", "bad work")
	if err == nil {
		t.Fatal("rejectCompletion() expected error for non-poster")
	}
	if !strings.Contains(err.Error(), "only the poster can reject") {
		t.Errorf("error = %q, want to contain 'only the poster can reject'", err.Error())
	}
}

func TestRejectCompletion_RejectError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "poster-rig"})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "evidence")
	store.RejectCompletionErr = fmt.Errorf("reject store error")

	err := rejectCompletion(store, "w-abc", "poster-rig", "reason")
	if err == nil {
		t.Fatal("rejectCompletion() expected error when RejectCompletion fails")
	}
	if !strings.Contains(err.Error(), "reject store error") {
		t.Errorf("error = %q, want to contain 'reject store error'", err.Error())
	}
}
