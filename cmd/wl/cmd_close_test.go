package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestCloseWanted_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "my-rig"})
	_ = store.ClaimWanted("w-abc", "my-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "my-rig", "https://github.com/pr/1")

	err := closeWanted(store, "w-abc", "my-rig")
	if err != nil {
		t.Fatalf("closeWanted() error: %v", err)
	}

	item, _ := store.QueryWanted("w-abc")
	if item.Status != "completed" {
		t.Errorf("Status = %q, want %q", item.Status, "completed")
	}
}

func TestCloseWanted_NotInReview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status string
	}{
		{"open item", ""},
		{"claimed item", "claimed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeWLCommonsStore()
			item := &commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "my-rig"}
			if tt.status != "" {
				item.Status = tt.status
			}
			_ = store.InsertWanted(item)
			if tt.status == "claimed" {
				_ = store.ClaimWanted("w-abc", "my-rig")
			}

			err := closeWanted(store, "w-abc", "my-rig")
			if err == nil {
				t.Fatal("closeWanted() expected error for non-in_review item")
			}
			if !strings.Contains(err.Error(), "not in_review") {
				t.Errorf("error = %q, want to contain 'not in_review'", err.Error())
			}
		})
	}
}

func TestCloseWanted_WrongPoster(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", PostedBy: "poster-rig"})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "evidence")

	err := closeWanted(store, "w-abc", "other-rig")
	if err == nil {
		t.Fatal("closeWanted() expected error for non-poster")
	}
	if !strings.Contains(err.Error(), "only the poster can close") {
		t.Errorf("error = %q, want to contain 'only the poster can close'", err.Error())
	}
}

func TestCloseWanted_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	err := closeWanted(store, "w-nonexistent", "my-rig")
	if err == nil {
		t.Fatal("closeWanted() expected error for missing item")
	}
}

func TestCloseWanted_StoreError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", Status: "in_review", PostedBy: "my-rig"})
	store.CloseWantedErr = fmt.Errorf("close store error")

	err := closeWanted(store, "w-abc", "my-rig")
	if err == nil {
		t.Fatal("closeWanted() expected error when CloseWanted fails")
	}
	if !strings.Contains(err.Error(), "close store error") {
		t.Errorf("error = %q, want to contain 'close store error'", err.Error())
	}
}
