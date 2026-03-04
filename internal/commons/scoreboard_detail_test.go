package commons

import (
	"fmt"
	"strings"
	"testing"
)

func TestQueryScoreboardDetail_Basic(t *testing.T) {
	t.Parallel()
	db := &fakeDB{results: map[string]string{
		// Base scoreboard queries.
		"GROUP BY s.subject":   "subject,stamp_count,weighted_score,unique_towns,avg_quality,avg_reliability,avg_creativity\nalice,4,15,3,4.2,3.8,3.5\n",
		"stamp_id IS NOT NULL": "completed_by,completions\nalice,2\n",
		"s.skill_tags":         "subject,skill_tags\n",
		// Display names (used by base scoreboard).
		"display_name\nFROM rigs": "handle,display_name\nalice,Alice Chen\n",
		// Rig metadata.
		"registered_at": "handle,registered_at,rig_type\nalice,2024-01-15,human\n",
		// Severity counts.
		"GROUP BY subject, severity": "subject,severity,cnt\nalice,root,1\nalice,branch,2\nalice,leaf,1\n",
		// Individual stamps.
		"ORDER BY subject, created_at": "subject,author,severity,quality,reliability,skill_tags,message,created_at\nalice,bob,root,5,4,,great work,2024-06-01\nalice,charlie,leaf,3,3,,ok,2024-05-01\n",
		// Completion history.
		"FROM completions c": "completed_by,wanted_id,wanted_title,completed_at,validated_at\nalice,w-1,Fix bug,2024-06-01,2024-06-02\n",
		// Badges.
		"FROM badges": "rig_handle,badge_type,awarded_at\nalice,pioneer,2024-03-01\n",
	}}

	entries, err := QueryScoreboardDetail(db, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.RigHandle != "alice" {
		t.Errorf("rig_handle = %q, want alice", e.RigHandle)
	}
	if e.WeightedScore != 15 {
		t.Errorf("weighted_score = %d, want 15", e.WeightedScore)
	}
	if e.TrustTier != "contributor" {
		t.Errorf("trust_tier = %q, want contributor", e.TrustTier)
	}
	if e.RegisteredAt != "2024-01-15" {
		t.Errorf("registered_at = %q, want 2024-01-15", e.RegisteredAt)
	}
	if e.RigType != "human" {
		t.Errorf("rig_type = %q, want human", e.RigType)
	}
	if e.RootStamps != 1 {
		t.Errorf("root_stamps = %d, want 1", e.RootStamps)
	}
	if e.BranchStamps != 2 {
		t.Errorf("branch_stamps = %d, want 2", e.BranchStamps)
	}
	if e.LeafStamps != 1 {
		t.Errorf("leaf_stamps = %d, want 1", e.LeafStamps)
	}
	if len(e.Stamps) != 2 {
		t.Fatalf("stamps count = %d, want 2", len(e.Stamps))
	}
	if e.Stamps[0].Author != "bob" {
		t.Errorf("first stamp author = %q, want bob", e.Stamps[0].Author)
	}
	if len(e.CompletionHistory) != 1 {
		t.Fatalf("completion_history count = %d, want 1", len(e.CompletionHistory))
	}
	if e.CompletionHistory[0].WantedTitle != "Fix bug" {
		t.Errorf("wanted_title = %q, want Fix bug", e.CompletionHistory[0].WantedTitle)
	}
	if len(e.Badges) != 1 {
		t.Fatalf("badges count = %d, want 1", len(e.Badges))
	}
	if e.Badges[0].BadgeType != "pioneer" {
		t.Errorf("badge_type = %q, want pioneer", e.Badges[0].BadgeType)
	}
}

func TestQueryScoreboardDetail_Empty(t *testing.T) {
	t.Parallel()
	db := &fakeDB{results: map[string]string{
		"GROUP BY s.subject": "subject,stamp_count,weighted_score,unique_towns,avg_quality,avg_reliability,avg_creativity\n",
	}}
	entries, err := QueryScoreboardDetail(db, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("got %v, want nil", entries)
	}
}

func TestQueryScoreboardDetail_QueryError(t *testing.T) {
	t.Parallel()
	db := &fakeDB{err: fmt.Errorf("detail error")}
	_, err := QueryScoreboardDetail(db, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "detail error") {
		t.Errorf("error = %q, want to contain 'detail error'", err.Error())
	}
}

func TestQueryScoreboardDetail_EmptyNested(t *testing.T) {
	t.Parallel()
	db := &fakeDB{results: map[string]string{
		"GROUP BY s.subject":           "subject,stamp_count,weighted_score,unique_towns,avg_quality,avg_reliability,avg_creativity\nalice,1,3,1,3.0,3.0,2.5\n",
		"stamp_id IS NOT NULL":         "completed_by,completions\n",
		"s.skill_tags":                 "subject,skill_tags\n",
		"display_name\nFROM rigs":      "handle,display_name\n",
		"registered_at":                "handle,registered_at,rig_type\n",
		"GROUP BY subject, severity":   "subject,severity,cnt\n",
		"ORDER BY subject, created_at": "subject,author,severity,quality,reliability,skill_tags,message,created_at\n",
		"FROM completions c":           "completed_by,wanted_id,wanted_title,completed_at,validated_at\n",
		"FROM badges":                  "rig_handle,badge_type,awarded_at\n",
	}}

	entries, err := QueryScoreboardDetail(db, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	// Empty nested arrays should be non-nil empty slices.
	if entries[0].Stamps == nil {
		t.Error("stamps should be non-nil empty slice")
	}
	if entries[0].CompletionHistory == nil {
		t.Error("completion_history should be non-nil empty slice")
	}
	if entries[0].Badges == nil {
		t.Error("badges should be non-nil empty slice")
	}
}
