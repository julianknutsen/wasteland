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

func TestReviewCreatePRMutuallyExclusive(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		jsonOut, md, stat, createPR bool
	}{
		{"create-pr+json", true, false, false, true},
		{"create-pr+md", false, true, false, true},
		{"create-pr+stat", false, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", tc.jsonOut, tc.md, tc.stat, tc.createPR)
			if err == nil {
				t.Error("expected error for mutually exclusive flags")
			}
		})
	}
}

func TestReviewCreatePRRequiresBranch(t *testing.T) {
	err := runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "", false, false, false, true)
	if err == nil {
		t.Error("expected error for --create-pr without branch")
	}
	if err != nil && !strings.Contains(err.Error(), "--create-pr requires a branch") {
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

func TestCreateGitHubPR(t *testing.T) {
	baseFake := func() *fakeGitHubPRClient {
		return &fakeGitHubPRClient{
			prs:              map[string]fakePR{},
			GetRefSHA:        "abc123",
			GetCommitTreeSHA: "tree456",
			CreateBlobSHA:    "blob789",
			CreateTreeSHA:    "newtree",
			CreateCommitSHA:  "newcommit",
			CreatePRURL:      "https://github.com/upstream/repo/pull/42",
		}
	}

	tests := []struct {
		name          string
		setup         func(*fakeGitHubPRClient)
		existingPR    *fakePR // if set, FindPR returns this
		wantURL       string
		wantErr       string
		wantCreateRef int
		wantUpdateRef int
		wantCreatePR  int
		wantUpdatePR  int
	}{
		{
			name:          "new PR success",
			wantURL:       "https://github.com/upstream/repo/pull/42",
			wantCreateRef: 1,
			wantCreatePR:  1,
		},
		{
			name:          "existing PR updates body",
			existingPR:    &fakePR{URL: "https://github.com/upstream/repo/pull/7", Number: "7"},
			wantURL:       "https://github.com/upstream/repo/pull/7",
			wantCreateRef: 1,
			wantUpdatePR:  1,
		},
		{
			name: "ref exists falls back to UpdateRef",
			setup: func(f *fakeGitHubPRClient) {
				f.CreateRefErr = fmt.Errorf("ref already exists")
			},
			wantURL:       "https://github.com/upstream/repo/pull/42",
			wantCreateRef: 1,
			wantUpdateRef: 1,
			wantCreatePR:  1,
		},
		{
			name: "GetRef fails",
			setup: func(f *fakeGitHubPRClient) {
				f.GetRefErr = fmt.Errorf("not found")
			},
			wantErr: "getting fork HEAD",
		},
		{
			name: "CreateBlob fails",
			setup: func(f *fakeGitHubPRClient) {
				f.CreateBlobErr = fmt.Errorf("quota exceeded")
			},
			wantErr: "creating blob",
		},
		{
			name: "CreateRef and UpdateRef both fail",
			setup: func(f *fakeGitHubPRClient) {
				f.CreateRefErr = fmt.Errorf("ref exists")
				f.UpdateRefErr = fmt.Errorf("permission denied")
			},
			wantErr:       "creating/updating ref",
			wantCreateRef: 1,
			wantUpdateRef: 1,
		},
		{
			name: "CreatePR fails",
			setup: func(f *fakeGitHubPRClient) {
				f.CreatePRErr = fmt.Errorf("validation failed")
			},
			wantErr:       "creating PR",
			wantCreateRef: 1,
			wantCreatePR:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := baseFake()
			if tc.existingPR != nil {
				client.prs["myfork:wl/rig/w-123"] = *tc.existingPR
			}
			if tc.setup != nil {
				tc.setup(client)
			}

			var buf bytes.Buffer
			url, err := createGitHubPR(client, "upstream/repo", "myfork", "forkdb", "wl/rig/w-123", "[wl] Test", "## diff", &buf)

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
			if len(client.CreateRefCalls) != tc.wantCreateRef {
				t.Errorf("CreateRef calls = %d, want %d", len(client.CreateRefCalls), tc.wantCreateRef)
			}
			if len(client.UpdateRefCalls) != tc.wantUpdateRef {
				t.Errorf("UpdateRef calls = %d, want %d", len(client.UpdateRefCalls), tc.wantUpdateRef)
			}
			if len(client.CreatePRCalls) != tc.wantCreatePR {
				t.Errorf("CreatePR calls = %d, want %d", len(client.CreatePRCalls), tc.wantCreatePR)
			}
			if len(client.UpdatePRCalls) != tc.wantUpdatePR {
				t.Errorf("UpdatePR calls = %d, want %d", len(client.UpdatePRCalls), tc.wantUpdatePR)
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
