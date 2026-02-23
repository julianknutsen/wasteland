package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/commons"
)

func TestGenerateStampID_Format(t *testing.T) {
	t.Parallel()
	id := generateStampID("w-abc123", "my-rig")
	if !strings.HasPrefix(id, "s-") {
		t.Errorf("generateStampID() = %q, want prefix 's-'", id)
	}
	// "s-" + 16 hex chars = 18 chars total
	if len(id) != 18 {
		t.Errorf("generateStampID() length = %d, want 18", len(id))
	}
	hexPart := id[2:]
	for _, c := range hexPart {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("generateStampID() contains non-hex char %q in %q", string(c), id)
		}
	}
}

func TestValidateAcceptInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		quality     int
		reliability int
		severity    string
		wantErr     string
	}{
		{"valid", 3, 4, "leaf", ""},
		{"quality too low", 0, 3, "leaf", "invalid quality"},
		{"quality too high", 6, 3, "leaf", "invalid quality"},
		{"reliability too low", 3, 0, "leaf", "invalid reliability"},
		{"reliability too high", 3, 6, "leaf", "invalid reliability"},
		{"bad severity", 3, 3, "bad", "invalid severity"},
		{"valid branch", 5, 5, "branch", ""},
		{"valid root", 1, 1, "root", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAcceptInputs(tt.quality, tt.reliability, tt.severity)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateAcceptInputs() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("validateAcceptInputs() expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestAcceptCompletion_Success(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "https://github.com/pr/1")

	stamp, err := acceptCompletion(store, "w-abc", "reviewer-rig", 4, 3, "leaf", []string{"go", "auth"}, "solid work")
	if err != nil {
		t.Fatalf("acceptCompletion() error: %v", err)
	}

	if stamp.Quality != 4 {
		t.Errorf("stamp.Quality = %d, want 4", stamp.Quality)
	}
	if stamp.Reliability != 3 {
		t.Errorf("stamp.Reliability = %d, want 3", stamp.Reliability)
	}
	if stamp.Severity != "leaf" {
		t.Errorf("stamp.Severity = %q, want %q", stamp.Severity, "leaf")
	}
	if stamp.Subject != "worker-rig" {
		t.Errorf("stamp.Subject = %q, want %q", stamp.Subject, "worker-rig")
	}
	if stamp.Author != "reviewer-rig" {
		t.Errorf("stamp.Author = %q, want %q", stamp.Author, "reviewer-rig")
	}
	if stamp.Message != "solid work" {
		t.Errorf("stamp.Message = %q, want %q", stamp.Message, "solid work")
	}
	if len(stamp.SkillTags) != 2 || stamp.SkillTags[0] != "go" || stamp.SkillTags[1] != "auth" {
		t.Errorf("stamp.SkillTags = %v, want [go auth]", stamp.SkillTags)
	}

	item, _ := store.QueryWanted("w-abc")
	if item.Status != "completed" {
		t.Errorf("Status = %q, want %q", item.Status, "completed")
	}
}

func TestAcceptCompletion_NotInReview(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})

	_, err := acceptCompletion(store, "w-abc", "reviewer-rig", 4, 3, "leaf", nil, "")
	if err == nil {
		t.Fatal("acceptCompletion() expected error for non-in_review item")
	}
	if !strings.Contains(err.Error(), "not in_review") {
		t.Errorf("error = %q, want to contain 'not in_review'", err.Error())
	}
}

func TestAcceptCompletion_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	_, err := acceptCompletion(store, "w-nonexistent", "reviewer-rig", 4, 3, "leaf", nil, "")
	if err == nil {
		t.Fatal("acceptCompletion() expected error for missing item")
	}
}

func TestAcceptCompletion_SelfAccept(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})
	_ = store.ClaimWanted("w-abc", "my-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "my-rig", "evidence")

	_, err := acceptCompletion(store, "w-abc", "my-rig", 4, 3, "leaf", nil, "")
	if err == nil {
		t.Fatal("acceptCompletion() expected error for self-accept")
	}
	if !strings.Contains(err.Error(), "cannot accept your own completion") {
		t.Errorf("error = %q, want to contain 'cannot accept your own completion'", err.Error())
	}
}

func TestAcceptCompletion_QueryCompletionError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug", Status: "in_review"})
	store.QueryCompletionErr = fmt.Errorf("completion query error")

	_, err := acceptCompletion(store, "w-abc", "reviewer-rig", 4, 3, "leaf", nil, "")
	if err == nil {
		t.Fatal("acceptCompletion() expected error when QueryCompletion fails")
	}
	if !strings.Contains(err.Error(), "completion query error") {
		t.Errorf("error = %q, want to contain 'completion query error'", err.Error())
	}
}

func TestAcceptCompletion_AcceptCompletionError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{ID: "w-abc", Title: "Fix bug"})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "evidence")
	store.AcceptCompletionErr = fmt.Errorf("accept store error")

	_, err := acceptCompletion(store, "w-abc", "reviewer-rig", 4, 3, "leaf", nil, "")
	if err == nil {
		t.Fatal("acceptCompletion() expected error when AcceptCompletion fails")
	}
	if !strings.Contains(err.Error(), "accept store error") {
		t.Errorf("error = %q, want to contain 'accept store error'", err.Error())
	}
}
