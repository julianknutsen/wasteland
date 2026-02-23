package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/commons"
)

func TestClaimWanted_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc123",
		Title: "Fix auth bug",
	})

	item, err := claimWanted(store, "w-abc123", "my-rig")
	if err != nil {
		t.Fatalf("claimWanted() error: %v", err)
	}
	if item.Title != "Fix auth bug" {
		t.Errorf("Title = %q, want %q", item.Title, "Fix auth bug")
	}

	// Verify status was updated in store
	updated, _ := store.QueryWanted("w-abc123")
	if updated.Status != "claimed" {
		t.Errorf("Status = %q, want %q", updated.Status, "claimed")
	}
	if updated.ClaimedBy != "my-rig" {
		t.Errorf("ClaimedBy = %q, want %q", updated.ClaimedBy, "my-rig")
	}
}

func TestClaimWanted_StoreError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	store.ClaimWantedErr = fmt.Errorf("claim store error")
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc123",
		Title: "Fix auth bug",
	})

	_, err := claimWanted(store, "w-abc123", "my-rig")
	if err == nil {
		t.Fatal("claimWanted() expected error when ClaimWanted fails")
	}
	if !strings.Contains(err.Error(), "claim store error") {
		t.Errorf("error = %q, want to contain 'claim store error'", err.Error())
	}
}

func TestClaimWanted_NotOpen(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix auth bug",
		Status: "claimed",
	})

	_, err := claimWanted(store, "w-abc123", "my-rig")
	if err == nil {
		t.Fatal("claimWanted() expected error for non-open item")
	}
}

func TestClaimWanted_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	_, err := claimWanted(store, "w-nonexistent", "my-rig")
	if err == nil {
		t.Fatal("claimWanted() expected error for missing item")
	}
}
