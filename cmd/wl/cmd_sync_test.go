package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWLCommonsFork_NotFound(t *testing.T) {
	// With no fork anywhere, should return empty
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	got := findWLCommonsFork()
	if got != "" {
		t.Errorf("findWLCommonsFork() = %q, want empty", got)
	}
}

func TestFindWLCommonsFork_InDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	forkDir := filepath.Join(tmpDir, "wasteland", "wl-commons")
	doltDir := filepath.Join(forkDir, ".dolt")
	if err := os.MkdirAll(doltDir, 0755); err != nil {
		t.Fatal(err)
	}

	got := findWLCommonsFork()
	if got != forkDir {
		t.Errorf("findWLCommonsFork() = %q, want %q", got, forkDir)
	}
}

func TestFindWLCommonsFork_NoDoltDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("HOME", t.TempDir())

	// Directory exists but has no .dolt subdirectory
	forkDir := filepath.Join(tmpDir, "wasteland", "wl-commons")
	if err := os.MkdirAll(forkDir, 0755); err != nil {
		t.Fatal(err)
	}

	got := findWLCommonsFork()
	if got != "" {
		t.Errorf("findWLCommonsFork() = %q, want empty (no .dolt dir)", got)
	}
}
