package main

import (
	"bytes"
	"testing"

	"github.com/julianknutsen/wasteland/internal/federation"
)

func TestMutationContext_WildWest(t *testing.T) {
	cfg := &federation.Config{
		Upstream:  "org/db",
		LocalDir:  "/tmp/fake",
		RigHandle: "test-rig",
		Mode:      "", // defaults to wild-west
	}

	mc := newMutationContext(cfg, "w-abc123", true, &bytes.Buffer{})

	if mc.BranchName() != "" {
		t.Errorf("BranchName() = %q, want empty in wild-west mode", mc.BranchName())
	}

	cleanup, err := mc.Setup()
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	// cleanup should be a no-op in wild-west mode.
	cleanup()
}

func TestMutationContext_WildWestExplicit(t *testing.T) {
	cfg := &federation.Config{
		Upstream:  "org/db",
		LocalDir:  "/tmp/fake",
		RigHandle: "test-rig",
		Mode:      federation.ModeWildWest,
	}

	mc := newMutationContext(cfg, "w-abc123", true, &bytes.Buffer{})

	if mc.BranchName() != "" {
		t.Errorf("BranchName() = %q, want empty in wild-west mode", mc.BranchName())
	}
}

func TestMutationContext_PRMode_BranchName(t *testing.T) {
	cfg := &federation.Config{
		Upstream:  "org/db",
		LocalDir:  "/tmp/fake",
		RigHandle: "test-rig",
		Mode:      federation.ModePR,
	}

	mc := newMutationContext(cfg, "w-abc123", true, &bytes.Buffer{})

	want := "wl/test-rig/w-abc123"
	if mc.BranchName() != want {
		t.Errorf("BranchName() = %q, want %q", mc.BranchName(), want)
	}
}
