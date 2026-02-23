package main

import (
	"bytes"
	"testing"
)

func TestRootCommand_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("run(nil) exit code = %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Error("expected help output on stdout")
	}
}

func TestRootCommand_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("run(nonexistent) exit code = %d, want 1", code)
	}
}

func TestSubcommandRegistration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	expected := []string{"join", "post", "claim", "unclaim", "done", "accept", "reject", "update", "delete", "browse", "status", "sync", "leave", "list", "config", "review", "merge", "version"}
	for _, name := range expected {
		found := false
		for _, c := range root.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("subcommand %q not found on root command", name)
		}
	}
}

func TestJoinAcceptsOptionalArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "join" {
			if err := c.Args(c, []string{}); err != nil {
				t.Errorf("join should accept 0 arguments (defaults to hop/wl-commons): %v", err)
			}
			if err := c.Args(c, []string{"org/db"}); err != nil {
				t.Errorf("join should accept 1 argument: %v", err)
			}
			if err := c.Args(c, []string{"a", "b"}); err == nil {
				t.Error("join should reject 2 arguments")
			}
			return
		}
	}
	t.Fatal("join command not found")
}

func TestClaimRequiresArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "claim" {
			if err := c.Args(c, []string{}); err == nil {
				t.Error("claim should require exactly 1 argument")
			}
			if err := c.Args(c, []string{"w-abc123"}); err != nil {
				t.Errorf("claim should accept 1 argument: %v", err)
			}
			return
		}
	}
	t.Fatal("claim command not found")
}

func TestDoneRequiresArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "done" {
			if err := c.Args(c, []string{}); err == nil {
				t.Error("done should require exactly 1 argument")
			}
			if err := c.Args(c, []string{"w-abc123"}); err != nil {
				t.Errorf("done should accept 1 argument: %v", err)
			}
			return
		}
	}
	t.Fatal("done command not found")
}

func TestBrowseNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "browse" {
			if err := c.Args(c, []string{}); err != nil {
				t.Errorf("browse should accept 0 arguments: %v", err)
			}
			return
		}
	}
	t.Fatal("browse command not found")
}

func TestSyncNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "sync" {
			if err := c.Args(c, []string{}); err != nil {
				t.Errorf("sync should accept 0 arguments: %v", err)
			}
			return
		}
	}
	t.Fatal("sync command not found")
}

func TestVersionOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("run(version) exit code = %d, want 0", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("wl")) {
		t.Errorf("version output = %q, want to contain 'wl'", stdout.String())
	}
}
