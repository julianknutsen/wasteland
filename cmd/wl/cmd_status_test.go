package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestGetStatus_Open(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:       "w-abc",
		Title:    "Fix the login bug",
		Type:     "bug",
		Priority: 1,
		PostedBy: "poster-rig",
	})

	result, err := getStatus(store, "w-abc")
	if err != nil {
		t.Fatalf("getStatus() error: %v", err)
	}
	if result.Item.Status != "open" {
		t.Errorf("Status = %q, want %q", result.Item.Status, "open")
	}
	if result.Completion != nil {
		t.Error("Completion should be nil for open item")
	}
	if result.Stamp != nil {
		t.Error("Stamp should be nil for open item")
	}
}

func TestGetStatus_Claimed(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.ClaimWanted("w-abc", "worker-rig")

	result, err := getStatus(store, "w-abc")
	if err != nil {
		t.Fatalf("getStatus() error: %v", err)
	}
	if result.Item.Status != "claimed" {
		t.Errorf("Status = %q, want %q", result.Item.Status, "claimed")
	}
	if result.Item.ClaimedBy != "worker-rig" {
		t.Errorf("ClaimedBy = %q, want %q", result.Item.ClaimedBy, "worker-rig")
	}
	if result.Completion != nil {
		t.Error("Completion should be nil for claimed item")
	}
	if result.Stamp != nil {
		t.Error("Stamp should be nil for claimed item")
	}
}

func TestGetStatus_InReview(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "https://github.com/pr/1")

	result, err := getStatus(store, "w-abc")
	if err != nil {
		t.Fatalf("getStatus() error: %v", err)
	}
	if result.Item.Status != "in_review" {
		t.Errorf("Status = %q, want %q", result.Item.Status, "in_review")
	}
	if result.Completion == nil {
		t.Fatal("Completion should not be nil for in_review item")
	}
	if result.Completion.ID != "c-test123" {
		t.Errorf("Completion.ID = %q, want %q", result.Completion.ID, "c-test123")
	}
	if result.Completion.Evidence != "https://github.com/pr/1" {
		t.Errorf("Completion.Evidence = %q, want %q", result.Completion.Evidence, "https://github.com/pr/1")
	}
	if result.Stamp != nil {
		t.Error("Stamp should be nil for in_review item")
	}
}

func TestGetStatus_Completed(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.ClaimWanted("w-abc", "worker-rig")
	_ = store.SubmitCompletion("c-test123", "w-abc", "worker-rig", "https://github.com/pr/1")
	_ = store.AcceptCompletion("w-abc", "c-test123", "reviewer-rig", &commons.Stamp{
		ID:          "s-stamp123",
		Author:      "reviewer-rig",
		Subject:     "worker-rig",
		Quality:     4,
		Reliability: 3,
		Severity:    "leaf",
		SkillTags:   []string{"go", "auth"},
		Message:     "solid work",
	})

	result, err := getStatus(store, "w-abc")
	if err != nil {
		t.Fatalf("getStatus() error: %v", err)
	}
	if result.Item.Status != "completed" {
		t.Errorf("Status = %q, want %q", result.Item.Status, "completed")
	}
	if result.Completion == nil {
		t.Fatal("Completion should not be nil for completed item")
	}
	if result.Stamp == nil {
		t.Fatal("Stamp should not be nil for completed item")
	}
	if result.Stamp.Quality != 4 {
		t.Errorf("Stamp.Quality = %d, want 4", result.Stamp.Quality)
	}
	if result.Stamp.Reliability != 3 {
		t.Errorf("Stamp.Reliability = %d, want 3", result.Stamp.Reliability)
	}
	if result.Stamp.Severity != "leaf" {
		t.Errorf("Stamp.Severity = %q, want %q", result.Stamp.Severity, "leaf")
	}
	if result.Stamp.Message != "solid work" {
		t.Errorf("Stamp.Message = %q, want %q", result.Stamp.Message, "solid work")
	}
	if len(result.Stamp.SkillTags) != 2 || result.Stamp.SkillTags[0] != "go" || result.Stamp.SkillTags[1] != "auth" {
		t.Errorf("Stamp.SkillTags = %v, want [go auth]", result.Stamp.SkillTags)
	}
}

func TestGetStatus_Withdrawn(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-abc",
		Title: "Fix bug",
	})
	_ = store.DeleteWanted("w-abc")

	result, err := getStatus(store, "w-abc")
	if err != nil {
		t.Fatalf("getStatus() error: %v", err)
	}
	if result.Item.Status != "withdrawn" {
		t.Errorf("Status = %q, want %q", result.Item.Status, "withdrawn")
	}
	if result.Completion != nil {
		t.Error("Completion should be nil for withdrawn item")
	}
	if result.Stamp != nil {
		t.Error("Stamp should be nil for withdrawn item")
	}
}

func TestGetStatus_NotFound(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()

	_, err := getStatus(store, "w-nonexistent")
	if err == nil {
		t.Fatal("getStatus() expected error for missing item")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestGetStatus_QueryError(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	store.QueryWantedDetailErr = fmt.Errorf("database connection failed")

	_, err := getStatus(store, "w-abc")
	if err == nil {
		t.Fatal("getStatus() expected error when query fails")
	}
	if !strings.Contains(err.Error(), "database connection failed") {
		t.Errorf("error = %q, want to contain 'database connection failed'", err.Error())
	}
}

func TestRenderStatus_Open(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	renderStatus(&buf, &StatusResult{
		Item: &commons.WantedItem{
			ID:          "w-abc123",
			Title:       "Fix the login bug",
			Status:      "open",
			Type:        "bug",
			Priority:    1,
			Project:     "gastown",
			EffortLevel: "medium",
			PostedBy:    "poster-rig",
			Tags:        []string{"go", "auth"},
			CreatedAt:   "2026-02-20 14:30:05",
			UpdatedAt:   "2026-02-20 14:30:05",
			Description: "The login page crashes.",
		},
	})

	out := buf.String()
	if !strings.Contains(out, "w-abc123") {
		t.Errorf("output missing wanted ID")
	}
	if !strings.Contains(out, "Fix the login bug") {
		t.Errorf("output missing title")
	}
	if !strings.Contains(out, "open") {
		t.Errorf("output missing status")
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output missing type")
	}
	if !strings.Contains(out, "P1") {
		t.Errorf("output missing priority")
	}
	if !strings.Contains(out, "gastown") {
		t.Errorf("output missing project")
	}
	if !strings.Contains(out, "poster-rig") {
		t.Errorf("output missing posted by")
	}
	if !strings.Contains(out, "go, auth") {
		t.Errorf("output missing tags")
	}
	if !strings.Contains(out, "The login page crashes.") {
		t.Errorf("output missing description")
	}
	// Should NOT contain completion or stamp sections
	if strings.Contains(out, "Completion:") {
		t.Errorf("output should not contain Completion section for open item")
	}
	if strings.Contains(out, "Stamp:") {
		t.Errorf("output should not contain Stamp section for open item")
	}
}

func TestRenderStatus_Completed(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	renderStatus(&buf, &StatusResult{
		Item: &commons.WantedItem{
			ID:          "w-abc123",
			Title:       "Fix the login bug",
			Status:      "completed",
			Type:        "bug",
			Priority:    1,
			Project:     "gastown",
			EffortLevel: "medium",
			PostedBy:    "poster-rig",
			Tags:        []string{"go", "auth"},
			ClaimedBy:   "worker-rig",
			CreatedAt:   "2026-02-20 14:30:05",
			UpdatedAt:   "2026-02-23 09:15:00",
		},
		Completion: &commons.CompletionRecord{
			ID:          "c-abc123def456ab",
			WantedID:    "w-abc123",
			CompletedBy: "worker-rig",
			Evidence:    "https://github.com/org/repo/pull/123",
		},
		Stamp: &commons.Stamp{
			ID:          "s-abc123def456ab",
			Author:      "reviewer-rig",
			Subject:     "worker-rig",
			Quality:     4,
			Reliability: 3,
			Severity:    "leaf",
			SkillTags:   []string{"go", "auth"},
			Message:     "solid work",
		},
	})

	out := buf.String()
	if !strings.Contains(out, "completed") {
		t.Errorf("output missing status")
	}
	if !strings.Contains(out, "Claimed by:") {
		t.Errorf("output missing claimed by")
	}
	if !strings.Contains(out, "c-abc123def456ab") {
		t.Errorf("output missing completion ID")
	}
	if !strings.Contains(out, "https://github.com/org/repo/pull/123") {
		t.Errorf("output missing evidence")
	}
	if !strings.Contains(out, "s-abc123def456ab") {
		t.Errorf("output missing stamp ID")
	}
	if !strings.Contains(out, "Quality: 4") {
		t.Errorf("output missing quality")
	}
	if !strings.Contains(out, "Reliability: 3") {
		t.Errorf("output missing reliability")
	}
	if !strings.Contains(out, "Severity: leaf") {
		t.Errorf("output missing severity")
	}
	if !strings.Contains(out, "go, auth") {
		t.Errorf("output missing skill tags")
	}
	if !strings.Contains(out, "Accepted by: reviewer-rig") {
		t.Errorf("output missing accepted by")
	}
	if !strings.Contains(out, "Message:     solid work") {
		t.Errorf("output missing message")
	}
}
