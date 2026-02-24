package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestDeleteWanted_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})

	err := deleteWanted(store, "w-abc")
	if err != nil {
		t.Fatalf("deleteWanted() error: %v", err)
	}

	item, _ := store.QueryWanted("w-abc")
	if item.Status != "withdrawn" {
		t.Errorf("Status = %q, want %q", item.Status, "withdrawn")
	}
}

func TestDeleteWanted_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	err := deleteWanted(store, "w-nonexistent")
	if err == nil {
		t.Fatal("deleteWanted() expected error for missing item")
	}
}

func TestDeleteWanted_NotOpen(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", Status: "claimed"})

	err := deleteWanted(store, "w-abc")
	if err == nil {
		t.Fatal("deleteWanted() expected error for non-open item")
	}
	if !strings.Contains(err.Error(), "not open") {
		t.Errorf("error = %q, want to contain 'not open'", err.Error())
	}
}

func TestDeleteWanted_NotOpenInReview(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", Status: "in_review"})

	err := deleteWanted(store, "w-abc")
	if err == nil {
		t.Fatal("deleteWanted() expected error for in_review item")
	}
	if !strings.Contains(err.Error(), "not open") {
		t.Errorf("error = %q, want to contain 'not open'", err.Error())
	}
}

func TestDeleteWanted_StoreError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})
	store.DeleteWantedErr = fmt.Errorf("delete store error")

	err := deleteWanted(store, "w-abc")
	if err == nil {
		t.Fatal("deleteWanted() expected error when DeleteWanted fails")
	}
	if !strings.Contains(err.Error(), "delete store error") {
		t.Errorf("error = %q, want to contain 'delete store error'", err.Error())
	}
}
