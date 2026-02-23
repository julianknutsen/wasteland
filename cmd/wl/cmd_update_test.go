package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/commons"
)

func TestValidateUpdateInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		itemType string
		effort   string
		priority int
		wantErr  string
	}{
		{"all empty", "", "", -1, ""},
		{"valid type", "bug", "", -1, ""},
		{"invalid type", "bad", "", -1, "invalid type"},
		{"valid effort", "", "small", -1, ""},
		{"invalid effort", "", "huge", -1, "invalid effort"},
		{"valid priority 0", "", "", 0, ""},
		{"valid priority 4", "", "", 4, ""},
		{"invalid priority too high", "", "", 9, "invalid priority"},
		{"invalid priority negative", "", "", -5, "invalid priority"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateUpdateInputs(tt.itemType, tt.effort, tt.priority)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateUpdateInputs() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("validateUpdateInputs() expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestHasUpdateFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		fields *commons.WantedUpdate
		want   bool
	}{
		{"empty", &commons.WantedUpdate{Priority: -1}, false},
		{"title set", &commons.WantedUpdate{Title: "new", Priority: -1}, true},
		{"priority set", &commons.WantedUpdate{Priority: 0}, true},
		{"tags set", &commons.WantedUpdate{Priority: -1, TagsSet: true}, true},
		{"effort set", &commons.WantedUpdate{Priority: -1, EffortLevel: "small"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasUpdateFields(tt.fields)
			if got != tt.want {
				t.Errorf("hasUpdateFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateWanted_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})

	fields := &commons.WantedUpdate{Title: "New title", Priority: -1}
	err := updateWanted(store, "w-abc", fields)
	if err != nil {
		t.Fatalf("updateWanted() error: %v", err)
	}

	item, _ := store.QueryWanted("w-abc")
	if item.Title != "New title" {
		t.Errorf("Title = %q, want %q", item.Title, "New title")
	}
}

func TestUpdateWanted_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	fields := &commons.WantedUpdate{Title: "New title", Priority: -1}
	err := updateWanted(store, "w-nonexistent", fields)
	if err == nil {
		t.Fatal("updateWanted() expected error for missing item")
	}
}

func TestUpdateWanted_NotOpen(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", Status: "claimed"})

	fields := &commons.WantedUpdate{Title: "New title", Priority: -1}
	err := updateWanted(store, "w-abc", fields)
	if err == nil {
		t.Fatal("updateWanted() expected error for non-open item")
	}
	if !strings.Contains(err.Error(), "not open") {
		t.Errorf("error = %q, want to contain 'not open'", err.Error())
	}
}

func TestUpdateWanted_StoreError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})
	store.UpdateWantedErr = fmt.Errorf("update store error")

	fields := &commons.WantedUpdate{Title: "New title", Priority: -1}
	err := updateWanted(store, "w-abc", fields)
	if err == nil {
		t.Fatal("updateWanted() expected error when UpdateWanted fails")
	}
	if !strings.Contains(err.Error(), "update store error") {
		t.Errorf("error = %q, want to contain 'update store error'", err.Error())
	}
}
