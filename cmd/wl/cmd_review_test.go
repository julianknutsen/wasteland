package main

import (
	"bytes"
	"fmt"
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

func TestSubmitPRReview(t *testing.T) {
	tests := []struct {
		name      string
		prs       map[string]fakePR
		submitErr error
		event     string
		wantURL   string
		wantErr   string
	}{
		{
			name:    "APPROVE success",
			prs:     map[string]fakePR{"myfork:wl/rig/w-123": {URL: "https://github.com/org/repo/pull/1", Number: "1"}},
			event:   "APPROVE",
			wantURL: "https://github.com/org/repo/pull/1",
		},
		{
			name:    "REQUEST_CHANGES success",
			prs:     map[string]fakePR{"myfork:wl/rig/w-123": {URL: "https://github.com/org/repo/pull/2", Number: "2"}},
			event:   "REQUEST_CHANGES",
			wantURL: "https://github.com/org/repo/pull/2",
		},
		{
			name:    "no PR found",
			prs:     map[string]fakePR{},
			event:   "APPROVE",
			wantErr: "no open PR",
		},
		{
			name:      "SubmitReview fails",
			prs:       map[string]fakePR{"myfork:wl/rig/w-123": {URL: "https://github.com/org/repo/pull/1", Number: "1"}},
			submitErr: fmt.Errorf("API error"),
			event:     "APPROVE",
			wantErr:   "submitting review",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeGitHubPRClient{
				prs:             tc.prs,
				SubmitReviewErr: tc.submitErr,
			}
			url, err := submitPRReview(client, "org/repo", "myfork", "wl/rig/w-123", tc.event, "looks good")
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q should contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if url != tc.wantURL {
				t.Errorf("got URL %q, want %q", url, tc.wantURL)
			}
		})
	}
}

func TestPRApprovalStatus(t *testing.T) {
	tests := []struct {
		name           string
		prs            map[string]fakePR
		reviews        map[string][]byte
		listReviewsErr error
		wantApproval   bool
		wantChangesReq bool
	}{
		{
			name:         "has approval",
			prs:          map[string]fakePR{"myfork:wl/rig/w-123": {Number: "1"}},
			reviews:      map[string][]byte{"1": []byte(`[{"user":{"login":"alice"},"state":"APPROVED"}]`)},
			wantApproval: true,
		},
		{
			name:           "has changes requested",
			prs:            map[string]fakePR{"myfork:wl/rig/w-123": {Number: "1"}},
			reviews:        map[string][]byte{"1": []byte(`[{"user":{"login":"alice"},"state":"CHANGES_REQUESTED"}]`)},
			wantChangesReq: true,
		},
		{
			name: "no PR found",
			prs:  map[string]fakePR{},
		},
		{
			name:           "ListReviews error",
			prs:            map[string]fakePR{"myfork:wl/rig/w-123": {Number: "1"}},
			listReviewsErr: fmt.Errorf("API error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeGitHubPRClient{
				prs:            tc.prs,
				reviews:        tc.reviews,
				ListReviewsErr: tc.listReviewsErr,
			}
			gotApproval, gotChangesReq := prApprovalStatus(client, "org/repo", "myfork", "wl/rig/w-123")
			if gotApproval != tc.wantApproval {
				t.Errorf("hasApproval = %v, want %v", gotApproval, tc.wantApproval)
			}
			if gotChangesReq != tc.wantChangesReq {
				t.Errorf("hasChangesRequested = %v, want %v", gotChangesReq, tc.wantChangesReq)
			}
		})
	}
}

func TestCloseGitHubPR(t *testing.T) {
	tests := []struct {
		name             string
		prs              map[string]fakePR
		closeErr         error
		deleteRefErr     error
		wantContains     []string
		wantNotContains  []string
		wantCloseCalls   int
		wantCommentCalls int
		wantDeleteCalls  int
	}{
		{
			name:             "full success",
			prs:              map[string]fakePR{"myfork:wl/rig/w-123": {URL: "https://github.com/org/repo/pull/1", Number: "1"}},
			wantContains:     []string{"Closed PR"},
			wantCloseCalls:   1,
			wantCommentCalls: 1,
			wantDeleteCalls:  1,
		},
		{
			name:            "no PR found",
			prs:             map[string]fakePR{},
			wantNotContains: []string{"Closed PR", "warning"},
		},
		{
			name:            "close fails",
			prs:             map[string]fakePR{"myfork:wl/rig/w-123": {URL: "https://github.com/org/repo/pull/1", Number: "1"}},
			closeErr:        fmt.Errorf("API error"),
			wantContains:    []string{"warning"},
			wantNotContains: []string{"Closed PR"},
			wantCloseCalls:  1,
		},
		{
			name:             "deleteRef fails",
			prs:              map[string]fakePR{"myfork:wl/rig/w-123": {URL: "https://github.com/org/repo/pull/1", Number: "1"}},
			deleteRefErr:     fmt.Errorf("ref error"),
			wantContains:     []string{"warning", "Closed PR"},
			wantCloseCalls:   1,
			wantCommentCalls: 1,
			wantDeleteCalls:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeGitHubPRClient{
				prs:          tc.prs,
				ClosePRErr:   tc.closeErr,
				DeleteRefErr: tc.deleteRefErr,
			}
			var buf bytes.Buffer
			closeGitHubPR(client, "org/repo", "myfork", "forkdb", "wl/rig/w-123", &buf)
			output := buf.String()
			for _, want := range tc.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output %q should contain %q", output, want)
				}
			}
			for _, notWant := range tc.wantNotContains {
				if strings.Contains(output, notWant) {
					t.Errorf("output %q should not contain %q", output, notWant)
				}
			}
			if len(client.ClosePRCalls) != tc.wantCloseCalls {
				t.Errorf("ClosePR calls = %d, want %d", len(client.ClosePRCalls), tc.wantCloseCalls)
			}
			if len(client.AddCommentCalls) != tc.wantCommentCalls {
				t.Errorf("AddComment calls = %d, want %d", len(client.AddCommentCalls), tc.wantCommentCalls)
			}
			if len(client.DeleteRefCalls) != tc.wantDeleteCalls {
				t.Errorf("DeleteRef calls = %d, want %d", len(client.DeleteRefCalls), tc.wantDeleteCalls)
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
