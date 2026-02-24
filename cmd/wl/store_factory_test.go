package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
)

// withFakeStore overrides the openStore factory for the duration of the test
// and returns the fake store for setup/assertions.
func withFakeStore(t *testing.T) *fakeWLCommonsStore {
	t.Helper()
	fake := newFakeWLCommonsStore()
	old := openStore
	openStore = func(string, bool, string) commons.WLCommonsStore { return fake }
	t.Cleanup(func() { openStore = old })
	return fake
}

// saveWasteland creates a minimal wasteland config on disk for handler tests.
func saveWasteland(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := federation.NewConfigStore()
	if err := store.Save(&federation.Config{
		Upstream:  "hop/wl-commons",
		ForkOrg:   "alice",
		ForkDB:    "wl-commons",
		LocalDir:  "/tmp/test/wl-commons",
		RigHandle: "alice",
		JoinedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("saving config: %v", err)
	}
}

func wastelandCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("wasteland", "", "")
	return cmd
}

// --- Handler-level tests using openStore factory ---

func TestRunStatus_Handler(t *testing.T) {
	saveWasteland(t)
	fake := withFakeStore(t)
	_ = fake.InsertWanted(&commons.WantedItem{
		ID: "w-handler", Title: "Handler test", Type: "bug", Priority: 1, PostedBy: "alice",
	})

	var stdout, stderr bytes.Buffer
	err := runStatus(wastelandCmd(), &stdout, &stderr, "w-handler")
	if err != nil {
		t.Fatalf("runStatus() error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "w-handler") {
		t.Errorf("output missing wanted ID: %q", out)
	}
	if !strings.Contains(out, "Handler test") {
		t.Errorf("output missing title: %q", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("output missing status: %q", out)
	}
}

func TestRunStatus_Handler_NotFound(t *testing.T) {
	saveWasteland(t)
	_ = withFakeStore(t)

	var stdout, stderr bytes.Buffer
	err := runStatus(wastelandCmd(), &stdout, &stderr, "w-nonexistent")
	if err == nil {
		t.Fatal("runStatus() expected error for missing item")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestRunStatus_Handler_NotJoined(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	err := runStatus(wastelandCmd(), &stdout, &stderr, "w-abc")
	if err == nil {
		t.Fatal("runStatus() expected error when not joined")
	}
}

func TestRunPost_Handler_WildWest(t *testing.T) {
	saveWasteland(t)
	fake := withFakeStore(t)

	var stdout, stderr bytes.Buffer
	err := runPost(wastelandCmd(), &stdout, &stderr,
		"Post handler test", "some description", "gastown", "bug",
		2, "medium", "go,test", true /* noPush */)
	if err != nil {
		t.Fatalf("runPost() error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Posted wanted item") {
		t.Errorf("output missing success message: %q", out)
	}
	if !strings.Contains(out, "Post handler test") {
		t.Errorf("output missing title: %q", out)
	}
	if !strings.Contains(out, "gastown") {
		t.Errorf("output missing project: %q", out)
	}

	// Verify the item was stored.
	if len(fake.items) != 1 {
		t.Errorf("expected 1 item in store, got %d", len(fake.items))
	}
}

func TestRunClaim_Handler_WildWest(t *testing.T) {
	saveWasteland(t)
	fake := withFakeStore(t)
	_ = fake.InsertWanted(&commons.WantedItem{
		ID: "w-claimtest", Title: "Claim me", PostedBy: "bob",
	})

	var stdout, stderr bytes.Buffer
	err := runClaim(wastelandCmd(), &stdout, &stderr, "w-claimtest", true /* noPush */)
	if err != nil {
		t.Fatalf("runClaim() error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Claimed w-claimtest") {
		t.Errorf("output missing success message: %q", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("output missing rig handle: %q", out)
	}

	// Verify state changed.
	item, _ := fake.QueryWanted("w-claimtest")
	if item.Status != "claimed" {
		t.Errorf("item status = %q, want %q", item.Status, "claimed")
	}
}

func TestRunDelete_Handler_WildWest(t *testing.T) {
	saveWasteland(t)
	fake := withFakeStore(t)
	_ = fake.InsertWanted(&commons.WantedItem{
		ID: "w-deltest", Title: "Delete me", PostedBy: "alice",
	})

	var stdout, stderr bytes.Buffer
	err := runDelete(wastelandCmd(), &stdout, &stderr, "w-deltest", true /* noPush */)
	if err != nil {
		t.Fatalf("runDelete() error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Withdrawn w-deltest") {
		t.Errorf("output missing success message: %q", out)
	}

	item, _ := fake.QueryWanted("w-deltest")
	if item.Status != "withdrawn" {
		t.Errorf("item status = %q, want %q", item.Status, "withdrawn")
	}
}

func TestRunUnclaim_Handler_WildWest(t *testing.T) {
	saveWasteland(t)
	fake := withFakeStore(t)
	_ = fake.InsertWanted(&commons.WantedItem{
		ID: "w-unclaimtest", Title: "Unclaim me", PostedBy: "alice",
	})
	_ = fake.ClaimWanted("w-unclaimtest", "alice")

	var stdout, stderr bytes.Buffer
	err := runUnclaim(wastelandCmd(), &stdout, &stderr, "w-unclaimtest", true /* noPush */)
	if err != nil {
		t.Fatalf("runUnclaim() error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Unclaimed w-unclaimtest") {
		t.Errorf("output missing success message: %q", out)
	}

	item, _ := fake.QueryWanted("w-unclaimtest")
	if item.Status != "open" {
		t.Errorf("item status = %q, want %q", item.Status, "open")
	}
}
