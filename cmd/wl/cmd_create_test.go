package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRequiresArg(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "create" {
			if err := c.Args(c, []string{}); err == nil {
				t.Error("create should require exactly 1 argument")
			}
			if err := c.Args(c, []string{"org/db"}); err != nil {
				t.Errorf("create should accept 1 argument: %v", err)
			}
			if err := c.Args(c, []string{"a", "b"}); err == nil {
				t.Error("create should reject 2 arguments")
			}
			return
		}
	}
	t.Fatal("create command not found")
}

func TestCreateInvalidUpstream(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := runCreate(&stdout, &stderr, "noslash", "", true, false)
	if err == nil {
		t.Fatal("expected error for invalid upstream")
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	// Create the fake .dolt directory at the expected path.
	doltDir := filepath.Join(dir, "wasteland", "org", "db", ".dolt")
	if err := os.MkdirAll(doltDir, 0o755); err != nil {
		t.Fatalf("creating fake .dolt dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runCreate(&stdout, &stderr, "org/db", "", true, false)
	if err == nil {
		t.Fatal("expected error when database already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}
