package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/wasteland/internal/federation"
)

func TestRunList_NoWastelands(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	var stdout, stderr bytes.Buffer
	err := runList(&stdout, &stderr)
	if err != nil {
		t.Fatalf("runList() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No wastelands joined") {
		t.Errorf("output = %q, want to contain 'No wastelands joined'", stdout.String())
	}
}

func TestRunList_SingleWasteland(t *testing.T) {
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

	var stdout, stderr bytes.Buffer
	err := runList(&stdout, &stderr)
	if err != nil {
		t.Fatalf("runList() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "hop/wl-commons") {
		t.Errorf("output missing upstream: %q", got)
	}
	if !strings.Contains(got, "alice") {
		t.Errorf("output missing handle: %q", got)
	}
	if !strings.Contains(got, "1") {
		t.Errorf("output missing count: %q", got)
	}
}

func TestRunList_MultipleWastelands(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	store := federation.NewConfigStore()
	for _, cfg := range []*federation.Config{
		{
			Upstream:  "hop/wl-commons",
			ForkOrg:   "alice",
			ForkDB:    "wl-commons",
			LocalDir:  "/tmp/test1",
			RigHandle: "alice",
			JoinedAt:  time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			Upstream:  "bob/wl-commons",
			ForkOrg:   "alice",
			ForkDB:    "wl-commons",
			LocalDir:  "/tmp/test2",
			RigHandle: "alice",
			JoinedAt:  time.Date(2025, 2, 20, 0, 0, 0, 0, time.UTC),
		},
	} {
		if err := store.Save(cfg); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
	}

	var stdout, stderr bytes.Buffer
	err := runList(&stdout, &stderr)
	if err != nil {
		t.Fatalf("runList() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "hop/wl-commons") {
		t.Errorf("output missing first upstream: %q", got)
	}
	if !strings.Contains(got, "bob/wl-commons") {
		t.Errorf("output missing second upstream: %q", got)
	}
	if !strings.Contains(got, "2") {
		t.Errorf("output missing count '2': %q", got)
	}
}

func TestRunList_CorruptConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create a valid config first so List() finds an upstream
	dir := filepath.Join(tmpDir, "wasteland", "wastelands", "hop")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write corrupt JSON
	if err := os.WriteFile(filepath.Join(dir, "wl-commons.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runList(&stdout, &stderr)
	if err != nil {
		t.Fatalf("runList() should not error on corrupt config: %v", err)
	}
	// Error should be printed to stderr
	if !strings.Contains(stderr.String(), "error loading config") {
		t.Errorf("stderr = %q, want error message about corrupt config", stderr.String())
	}
}
