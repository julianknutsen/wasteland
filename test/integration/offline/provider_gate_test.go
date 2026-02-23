//go:build integration

package offline

import (
	"strings"
	"testing"
)

func TestProviderGateApprove(t *testing.T) {
	for _, backend := range backends {
		if backend == githubBackend {
			continue // on github backend, approve passes the gate
		}
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			_, stderr, err := runWL(t, env, "approve", "wl/x/w-y")
			if err == nil {
				t.Fatal("approve should fail on non-GitHub backend")
			}
			if !strings.Contains(stderr, "requires GitHub provider") {
				t.Errorf("expected 'requires GitHub provider' error, got stderr: %s", stderr)
			}
		})
	}
}

func TestProviderGateRequestChanges(t *testing.T) {
	for _, backend := range backends {
		if backend == githubBackend {
			continue // on github backend, request-changes passes the gate
		}
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			_, stderr, err := runWL(t, env, "request-changes", "wl/x/w-y", "--comment", "needs work")
			if err == nil {
				t.Fatal("request-changes should fail on non-GitHub backend")
			}
			if !strings.Contains(stderr, "requires GitHub provider") {
				t.Errorf("expected 'requires GitHub provider' error, got stderr: %s", stderr)
			}
		})
	}
}

func TestProviderGateReviewGHPR(t *testing.T) {
	for _, backend := range backends {
		if backend == githubBackend {
			continue // on github backend, --gh-pr passes the gate
		}
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			// Switch to PR mode and post.
			setMode(t, env, upstream, "pr")

			stdout, _, err := runWL(t, env, "post",
				"--title", "Gate test item",
				"--type", "feature",
				"--no-push",
			)
			if err != nil {
				t.Fatalf("wl post failed: %v", err)
			}
			wantedID := extractWantedID(t, stdout)
			branch := "wl/" + forkOrg + "/" + wantedID

			_, stderr, err := runWL(t, env, "review", branch, "--gh-pr")
			if err == nil {
				t.Fatal("review --gh-pr should fail on non-GitHub backend")
			}
			if !strings.Contains(stderr, "requires GitHub provider") {
				t.Errorf("expected 'requires GitHub provider' error, got stderr: %s", stderr)
			}
		})
	}
}

func TestProviderGateApproveOnGitHub(t *testing.T) {
	for _, backend := range backends {
		if backend != githubBackend {
			continue // only test on github backend
		}
		t.Run(string(backend), func(t *testing.T) {
			env := joinedEnv(t, backend)

			// On the github backend, approve should pass the provider gate
			// and fail at a LATER point (gh CLI not found or no PR).
			_, stderr, err := runWL(t, env, "approve", "wl/x/w-y")
			if err == nil {
				t.Fatal("approve should still fail (no gh CLI / no PR), but not at the gate")
			}
			// The error should NOT be the provider gate error.
			if strings.Contains(stderr, "requires GitHub provider") {
				t.Errorf("github backend should pass provider gate, but got: %s", stderr)
			}
		})
	}
}
