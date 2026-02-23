package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReviewRequiresNoMoreThanOneArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "review" {
			if err := c.Args(c, []string{}); err != nil {
				t.Errorf("review should accept 0 arguments: %v", err)
			}
			if err := c.Args(c, []string{"wl/rig/w-abc"}); err != nil {
				t.Errorf("review should accept 1 argument: %v", err)
			}
			if err := c.Args(c, []string{"a", "b"}); err == nil {
				t.Error("review should reject 2 arguments")
			}
			return
		}
	}
	t.Fatal("review command not found")
}

func TestReviewMutuallyExclusiveFlags(t *testing.T) {
	err := runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", true, true, false, false)
	if err == nil {
		t.Error("expected error for --json + --md")
	}

	err = runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", true, false, true, false)
	if err == nil {
		t.Error("expected error for --json + --stat")
	}

	err = runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", false, true, true, false)
	if err == nil {
		t.Error("expected error for --md + --stat")
	}
}

func TestReviewGhPRMutuallyExclusive(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		jsonOut, md, stat, ghPR bool
	}{
		{"gh-pr+json", true, false, false, true},
		{"gh-pr+md", false, true, false, true},
		{"gh-pr+stat", false, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", tc.jsonOut, tc.md, tc.stat, tc.ghPR)
			if err == nil {
				t.Error("expected error for mutually exclusive flags")
			}
		})
	}
}

func TestReviewGhPRRequiresBranch(t *testing.T) {
	err := runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "", false, false, false, true)
	if err == nil {
		t.Error("expected error for --gh-pr without branch")
	}
	if err != nil && !strings.Contains(err.Error(), "--gh-pr requires a branch") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseReviewStatus(t *testing.T) {
	tests := []struct {
		name                         string
		json                         string
		wantApproval, wantChangesReq bool
	}{
		{
			name:         "empty reviews",
			json:         `[]`,
			wantApproval: false, wantChangesReq: false,
		},
		{
			name:         "single approval",
			json:         `[{"user":{"login":"alice"},"state":"APPROVED"}]`,
			wantApproval: true, wantChangesReq: false,
		},
		{
			name:         "single changes requested",
			json:         `[{"user":{"login":"alice"},"state":"CHANGES_REQUESTED"}]`,
			wantApproval: false, wantChangesReq: true,
		},
		{
			name: "changes then approval same user",
			json: `[
				{"user":{"login":"alice"},"state":"CHANGES_REQUESTED"},
				{"user":{"login":"alice"},"state":"APPROVED"}
			]`,
			wantApproval: true, wantChangesReq: false,
		},
		{
			name: "approval then changes same user",
			json: `[
				{"user":{"login":"alice"},"state":"APPROVED"},
				{"user":{"login":"alice"},"state":"CHANGES_REQUESTED"}
			]`,
			wantApproval: false, wantChangesReq: true,
		},
		{
			name: "mixed users",
			json: `[
				{"user":{"login":"alice"},"state":"APPROVED"},
				{"user":{"login":"bob"},"state":"CHANGES_REQUESTED"}
			]`,
			wantApproval: true, wantChangesReq: true,
		},
		{
			name:         "comment only ignored",
			json:         `[{"user":{"login":"alice"},"state":"COMMENTED"}]`,
			wantApproval: false, wantChangesReq: false,
		},
		{
			name:         "invalid JSON",
			json:         `not json`,
			wantApproval: false, wantChangesReq: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotApproval, gotChangesReq := parseReviewStatus([]byte(tc.json))
			if gotApproval != tc.wantApproval {
				t.Errorf("hasApproval = %v, want %v", gotApproval, tc.wantApproval)
			}
			if gotChangesReq != tc.wantChangesReq {
				t.Errorf("hasChangesRequested = %v, want %v", gotChangesReq, tc.wantChangesReq)
			}
		})
	}
}

func TestExtractWantedID(t *testing.T) {
	tests := []struct {
		branch, want string
	}{
		{"wl/myrig/w-abc123", "w-abc123"},
		{"wl/rig/w-xyz", "w-xyz"},
		{"nobranch", "nobranch"},
		{"one/two", "one/two"},
	}
	for _, tc := range tests {
		got := extractWantedID(tc.branch)
		if got != tc.want {
			t.Errorf("extractWantedID(%q) = %q, want %q", tc.branch, got, tc.want)
		}
	}
}
