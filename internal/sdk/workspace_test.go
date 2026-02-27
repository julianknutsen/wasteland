package sdk

import (
	"testing"
)

func TestWorkspace_RigHandle(t *testing.T) {
	ws := NewWorkspace("alice")
	if ws.RigHandle() != "alice" {
		t.Errorf("expected alice, got %s", ws.RigHandle())
	}
}

func TestWorkspace_AddAndClient(t *testing.T) {
	ws := NewWorkspace("alice")
	client := &Client{rigHandle: "alice", mode: "wild-west"}
	info := UpstreamInfo{
		Upstream: "hop/wl-commons",
		ForkOrg:  "alice-org",
		ForkDB:   "wl-commons",
		Mode:     "wild-west",
	}
	ws.Add(info, client)

	got, err := ws.Client("hop/wl-commons")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != client {
		t.Error("expected same client instance")
	}
}

func TestWorkspace_ClientNotFound(t *testing.T) {
	ws := NewWorkspace("alice")
	_, err := ws.Client("missing/upstream")
	if err == nil {
		t.Error("expected error for missing upstream")
	}
}

func TestWorkspace_Remove(t *testing.T) {
	ws := NewWorkspace("alice")
	client := &Client{rigHandle: "alice"}
	ws.Add(UpstreamInfo{Upstream: "hop/wl-commons"}, client)
	ws.Remove("hop/wl-commons")

	_, err := ws.Client("hop/wl-commons")
	if err == nil {
		t.Error("expected error after remove")
	}

	upstreams := ws.Upstreams()
	if len(upstreams) != 0 {
		t.Errorf("expected 0 upstreams after remove, got %d", len(upstreams))
	}
}

func TestWorkspace_Upstreams_Sorted(t *testing.T) {
	ws := NewWorkspace("alice")
	ws.Add(UpstreamInfo{Upstream: "z/repo", ForkOrg: "a", ForkDB: "b", Mode: "pr"}, &Client{})
	ws.Add(UpstreamInfo{Upstream: "a/repo", ForkOrg: "c", ForkDB: "d", Mode: "wild-west"}, &Client{})
	ws.Add(UpstreamInfo{Upstream: "m/repo", ForkOrg: "e", ForkDB: "f", Mode: "pr"}, &Client{})

	upstreams := ws.Upstreams()
	if len(upstreams) != 3 {
		t.Fatalf("expected 3 upstreams, got %d", len(upstreams))
	}
	if upstreams[0].Upstream != "a/repo" {
		t.Errorf("expected a/repo first, got %s", upstreams[0].Upstream)
	}
	if upstreams[1].Upstream != "m/repo" {
		t.Errorf("expected m/repo second, got %s", upstreams[1].Upstream)
	}
	if upstreams[2].Upstream != "z/repo" {
		t.Errorf("expected z/repo third, got %s", upstreams[2].Upstream)
	}
}

func TestWorkspace_Upstreams_Empty(t *testing.T) {
	ws := NewWorkspace("alice")
	upstreams := ws.Upstreams()
	if len(upstreams) != 0 {
		t.Errorf("expected 0 upstreams, got %d", len(upstreams))
	}
}
