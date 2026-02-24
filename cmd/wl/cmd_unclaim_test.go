package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestUnclaimWanted_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:       "w-abc123",
		Title:    "Fix auth bug",
		PostedBy: "poster-rig",
	})
	_ = store.ClaimWanted("w-abc123", "my-rig")

	item, err := unclaimWanted(store, "w-abc123", "my-rig")
	if err != nil {
		t.Fatalf("unclaimWanted() error: %v", err)
	}
	if item.Title != "Fix auth bug" {
		t.Errorf("Title = %q, want %q", item.Title, "Fix auth bug")
	}

	// Verify status was reverted in store.
	updated, _ := store.QueryWanted("w-abc123")
	if updated.Status != "open" {
		t.Errorf("Status = %q, want %q", updated.Status, "open")
	}
	if updated.ClaimedBy != "" {
		t.Errorf("ClaimedBy = %q, want empty", updated.ClaimedBy)
	}
}

func TestUnclaimWanted_ByPoster(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:       "w-abc123",
		Title:    "Fix auth bug",
		PostedBy: "poster-rig",
	})
	_ = store.ClaimWanted("w-abc123", "claimer-rig")

	// Poster (not claimer) unclaims — should succeed.
	item, err := unclaimWanted(store, "w-abc123", "poster-rig")
	if err != nil {
		t.Fatalf("unclaimWanted() error: %v", err)
	}
	if item.Title != "Fix auth bug" {
		t.Errorf("Title = %q, want %q", item.Title, "Fix auth bug")
	}

	updated, _ := store.QueryWanted("w-abc123")
	if updated.Status != "open" {
		t.Errorf("Status = %q, want %q", updated.Status, "open")
	}
	if updated.ClaimedBy != "" {
		t.Errorf("ClaimedBy = %q, want empty", updated.ClaimedBy)
	}
}

func TestUnclaimWanted_NotClaimed(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:       "w-abc123",
		Title:    "Fix auth bug",
		PostedBy: "poster-rig",
	})

	_, err := unclaimWanted(store, "w-abc123", "my-rig")
	if err == nil {
		t.Fatal("unclaimWanted() expected error for non-claimed item")
	}
	if !strings.Contains(err.Error(), "not claimed") {
		t.Errorf("error = %q, want to contain 'not claimed'", err.Error())
	}
}

func TestUnclaimWanted_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	_, err := unclaimWanted(store, "w-nonexistent", "my-rig")
	if err == nil {
		t.Fatal("unclaimWanted() expected error for missing item")
	}
}

func TestUnclaimWanted_NotAuthorized(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:       "w-abc123",
		Title:    "Fix auth bug",
		PostedBy: "poster-rig",
	})
	_ = store.ClaimWanted("w-abc123", "claimer-rig")

	// Someone who is neither claimer nor poster tries to unclaim.
	_, err := unclaimWanted(store, "w-abc123", "random-rig")
	if err == nil {
		t.Fatal("unclaimWanted() expected error for unauthorized rig")
	}
	if !strings.Contains(err.Error(), "only the claimer or poster") {
		t.Errorf("error = %q, want to contain 'only the claimer or poster'", err.Error())
	}
}

func TestUnclaimWanted_StoreError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	store.UnclaimWantedErr = fmt.Errorf("unclaim store error")
	_ = store.InsertWanted(&commons.WantedItem{
		ID:       "w-abc123",
		Title:    "Fix auth bug",
		PostedBy: "poster-rig",
	})
	_ = store.ClaimWanted("w-abc123", "my-rig")

	_, err := unclaimWanted(store, "w-abc123", "my-rig")
	if err == nil {
		t.Fatal("unclaimWanted() expected error when UnclaimWanted fails")
	}
	if !strings.Contains(err.Error(), "unclaim store error") {
		t.Errorf("error = %q, want to contain 'unclaim store error'", err.Error())
	}
}
