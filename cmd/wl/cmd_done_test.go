package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/commons"
)

func TestGenerateCompletionID_Format(t *testing.T) {
	t.Parallel()
	id := commons.GeneratePrefixedID("c", "w-abc123", "my-rig")
	if !strings.HasPrefix(id, "c-") {
		t.Errorf("GeneratePrefixedID(c) = %q, want prefix 'c-'", id)
	}
	// "c-" + 16 hex chars = 18 chars total
	if len(id) != 18 {
		t.Errorf("GeneratePrefixedID(c) length = %d, want 18", len(id))
	}
	hexPart := id[2:]
	for _, c := range hexPart {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("GeneratePrefixedID(c) contains non-hex char %q in %q", string(c), id)
		}
	}
}

func TestGenerateCompletionID_DeterministicInputs(t *testing.T) {
	t.Parallel()
	id1 := commons.GeneratePrefixedID("c", "w-abc", "rig-1")
	id2 := commons.GeneratePrefixedID("c", "w-def", "rig-1")
	id3 := commons.GeneratePrefixedID("c", "w-abc", "rig-2")

	if id1 == id2 {
		t.Errorf("same ID for different wantedIDs: %s", id1)
	}
	if id1 == id3 {
		t.Errorf("same ID for different rigHandles: %s", id1)
	}
}

func TestSubmitDone_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.ClaimWanted("w-abc", "my-rig")

	err := submitDone(store, "w-abc", "my-rig", "https://github.com/pr/1", "c-test123")
	if err != nil {
		t.Fatalf("submitDone() error: %v", err)
	}

	item, _ := store.QueryWanted("w-abc")
	if item.Status != "in_review" {
		t.Errorf("Status = %q, want %q", item.Status, "in_review")
	}
}

func TestSubmitDone_StoreError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	store.SubmitCompletionErr = fmt.Errorf("completion store error")
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.ClaimWanted("w-abc", "my-rig")

	err := submitDone(store, "w-abc", "my-rig", "https://github.com/pr/1", "c-test")
	if err == nil {
		t.Fatal("submitDone() expected error when SubmitCompletion fails")
	}
	if !strings.Contains(err.Error(), "completion store error") {
		t.Errorf("error = %q, want to contain 'completion store error'", err.Error())
	}
}

func TestSubmitDone_NotClaimed(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})

	err := submitDone(store, "w-abc", "my-rig", "evidence", "c-test")
	if err == nil {
		t.Fatal("submitDone() expected error for unclaimed item")
	}
}

func TestSubmitDone_WrongClaimer(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.ClaimWanted("w-abc", "other-rig")

	err := submitDone(store, "w-abc", "my-rig", "evidence", "c-test")
	if err == nil {
		t.Fatal("submitDone() expected error for wrong claimer")
	}
}

func TestSubmitDone_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	err := submitDone(store, "w-nonexistent", "my-rig", "evidence", "c-test")
	if err == nil {
		t.Fatal("submitDone() expected error for missing item")
	}
}
