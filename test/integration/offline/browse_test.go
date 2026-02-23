//go:build integration

package offline

import (
	"strings"
	"testing"
)

func TestBrowseIntegration(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := newTestEnv(t, backend)
			env.createUpstreamStoreWithData(t, upstreamOrg, upstreamDB)
			env.joinWasteland(t, upstream, forkOrg)

			stdout, stderr, err := runWL(t, env, "browse")
			if err != nil {
				t.Fatalf("wl browse failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			if !strings.Contains(stdout, "Seed item from upstream") {
				t.Errorf("expected browse output to contain 'Seed item from upstream', got: %s", stdout)
			}
		})
	}
}

func TestBrowseJSON(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := newTestEnv(t, backend)
			env.createUpstreamStoreWithData(t, upstreamOrg, upstreamDB)
			env.joinWasteland(t, upstream, forkOrg)

			stdout, stderr, err := runWL(t, env, "browse", "--json")
			if err != nil {
				t.Fatalf("wl browse --json failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			if !strings.Contains(stdout, "w-seed001") {
				t.Errorf("expected JSON output to contain 'w-seed001', got: %s", stdout)
			}
		})
	}
}

func TestBrowseFilterByType(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := newTestEnv(t, backend)
			env.createUpstreamStoreWithData(t, upstreamOrg, upstreamDB)
			env.joinWasteland(t, upstream, forkOrg)

			// Seed data has type=feature. Filter by --type bug.
			stdout, stderr, err := runWL(t, env, "browse", "--type", "bug")
			if err != nil {
				t.Fatalf("wl browse --type bug failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			if strings.Contains(stdout, "Seed item from upstream") {
				t.Errorf("expected seed item NOT in bug-filtered output, got: %s", stdout)
			}
		})
	}
}
