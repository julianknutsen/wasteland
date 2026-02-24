package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/spf13/cobra"
)

func TestRunLeave_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	store := federation.NewConfigStore()
	cfg := &federation.Config{
		Upstream:  "hop/wl-commons",
		ForkOrg:   "alice",
		ForkDB:    "wl-commons",
		LocalDir:  "/tmp/test/wl-commons",
		RigHandle: "alice",
		JoinedAt:  time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Build a minimal cobra command with the --wasteland flag
	cmd := &cobra.Command{}
	cmd.Flags().String("wasteland", "", "")

	var stdout, stderr bytes.Buffer
	err := runLeave(cmd, &stdout, &stderr, "hop/wl-commons")
	if err != nil {
		t.Fatalf("runLeave() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Left wasteland") {
		t.Errorf("output missing 'Left wasteland': %q", got)
	}
	if !strings.Contains(got, "hop/wl-commons") {
		t.Errorf("output missing upstream: %q", got)
	}

	// Verify config was deleted
	_, err = store.Load("hop/wl-commons")
	if err == nil {
		t.Error("config should be deleted after leave")
	}
}

func TestRunLeave_NotJoined(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cmd := &cobra.Command{}
	cmd.Flags().String("wasteland", "", "")

	var stdout, stderr bytes.Buffer
	err := runLeave(cmd, &stdout, &stderr, "hop/wl-commons")
	if err == nil {
		t.Fatal("runLeave() expected error for non-joined wasteland")
	}
}

func TestRunLeave_AutoResolvesSingleWasteland(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	store := federation.NewConfigStore()
	cfg := &federation.Config{
		Upstream:  "hop/wl-commons",
		ForkOrg:   "alice",
		ForkDB:    "wl-commons",
		LocalDir:  "/tmp/test/wl-commons",
		RigHandle: "alice",
		JoinedAt:  time.Now(),
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// No positional arg, no --wasteland flag — should auto-resolve
	cmd := &cobra.Command{}
	cmd.Flags().String("wasteland", "", "")

	var stdout, stderr bytes.Buffer
	err := runLeave(cmd, &stdout, &stderr, "")
	if err != nil {
		t.Fatalf("runLeave() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Left wasteland") {
		t.Errorf("output missing 'Left wasteland': %q", stdout.String())
	}
}
