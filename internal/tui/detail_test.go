package tui

import (
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestDetail_PendingLine_ShowsWhenBranchDiffers(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.refreshViewport()

	v := m.View()
	if !strings.Contains(v, "Pending:") {
		t.Errorf("view should contain 'Pending:' line, got:\n%s", v)
	}
	if !strings.Contains(v, "open → claimed") {
		t.Errorf("view should contain 'open → claimed', got:\n%s", v)
	}
}

func TestDetail_PendingLine_HiddenWhenNoMainStatus(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = ""
	m.detail.mainStatus = ""
	m.detail.refreshViewport()

	v := m.View()
	if strings.Contains(v, "Pending:") {
		t.Errorf("view should NOT contain 'Pending:' without branch, got:\n%s", v)
	}
}

func TestDetail_PendingLine_HiddenWhenStatusesSame(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open" // same as item status
	m.detail.refreshViewport()

	v := m.View()
	if strings.Contains(v, "Pending:") {
		t.Errorf("view should NOT contain 'Pending:' when statuses match, got:\n%s", v)
	}
}

func TestDetail_DeltaHints_PRMode_SubmitAndDiscard(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	hints := m.detail.actionHints()
	// PR mode without existing PR: M opens submit PR view.
	if !strings.Contains(hints, "M:submit PR") {
		t.Errorf("PR mode hints should contain 'M:submit PR', got: %q", hints)
	}
	if !strings.Contains(hints, "b:discard (→ open)") {
		t.Errorf("hints should contain 'b:discard (→ open)', got: %q", hints)
	}
}

func TestDetail_DeltaHints_PRMode_ExistingPR_HidesSubmit(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.prURL = "https://github.com/org/repo/pull/42"

	hints := m.detail.actionHints()
	// PR already exists: submit hint should NOT appear.
	if strings.Contains(hints, "M:submit") {
		t.Errorf("hints should NOT contain submit when PR exists, got: %q", hints)
	}
	// Discard should still be available.
	if !strings.Contains(hints, "b:discard (→ open)") {
		t.Errorf("hints should contain 'b:discard (→ open)', got: %q", hints)
	}
}

func TestDetail_DeltaHints_WildWest_ApplyAndDiscard(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	hints := m.detail.actionHints()
	if !strings.Contains(hints, "M:apply claim") {
		t.Errorf("wild-west hints should contain 'M:apply claim', got: %q", hints)
	}
	if !strings.Contains(hints, "b:discard (→ open)") {
		t.Errorf("hints should contain 'b:discard (→ open)', got: %q", hints)
	}
}

func TestDetail_DeltaHints_NotShownWithoutBranch(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = ""
	m.detail.mainStatus = ""

	hints := m.detail.actionHints()
	if strings.Contains(hints, "M:apply") {
		t.Errorf("hints should NOT contain delta actions without branch, got: %q", hints)
	}
}

func TestDetail_DeltaHints_MultiHop_WildWest(t *testing.T) {
	// Simulate multi-hop: main=open, branch=in_review
	m := newDetailForTest("in_review", "test-rig", "other-rig", "wild-west")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	hints := m.detail.actionHints()
	if !strings.Contains(hints, "M:apply changes") {
		t.Errorf("multi-hop should show 'M:apply changes', got: %q", hints)
	}
	if !strings.Contains(hints, "b:discard (→ open)") {
		t.Errorf("multi-hop should show 'b:discard (→ open)', got: %q", hints)
	}
}

func TestDetail_TryDelta_NoBranch_NoOp(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")
	// No branch set.

	result, cmd := m.Update(keyMsg("M"))
	m2 := result.(Model)
	_ = m2
	if cmd != nil {
		t.Error("M key without branch should not return a cmd")
	}

	result, cmd = m.Update(keyMsg("b"))
	m2 = result.(Model)
	_ = m2
	if cmd != nil {
		t.Error("b key without branch should not return a cmd")
	}
}

func TestDetail_TryDelta_Apply_PRMode_OpensSubmit(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	// M key in PR mode opens the submit PR view.
	result, cmd := m.Update(keyMsg("M"))
	m2 := result.(Model)
	if m2.detail.submit == nil {
		t.Fatal("M key in PR mode should open submit view")
	}
	if m2.detail.submit.branch != "wl/test-rig/w-abc123" {
		t.Errorf("submit branch = %q, want %q", m2.detail.submit.branch, "wl/test-rig/w-abc123")
	}
	// Should return a cmd to trigger diff loading.
	if cmd == nil {
		t.Fatal("M key should return submitOpenedMsg cmd")
	}
	msg := cmd()
	opened, ok := msg.(submitOpenedMsg)
	if !ok {
		t.Fatalf("expected submitOpenedMsg, got %T", msg)
	}
	if opened.branch != "wl/test-rig/w-abc123" {
		t.Errorf("submitOpenedMsg branch = %q, want %q", opened.branch, "wl/test-rig/w-abc123")
	}
}

func TestDetail_TryDelta_Apply_PRMode_ExistingPR_ShowsURL(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.prURL = "https://github.com/org/repo/pull/42"

	// M key when PR exists should NOT open submit view.
	result, cmd := m.Update(keyMsg("M"))
	m2 := result.(Model)
	if m2.detail.submit != nil {
		t.Error("M key should NOT open submit when PR already exists")
	}
	if cmd != nil {
		t.Error("should not return a cmd when PR already exists")
	}
	if !strings.Contains(m2.detail.result, "PR already open") {
		t.Errorf("result should indicate PR already open, got: %q", m2.detail.result)
	}
	if !strings.Contains(m2.detail.result, "https://github.com/org/repo/pull/42") {
		t.Errorf("result should contain PR URL, got: %q", m2.detail.result)
	}
}

func TestDetail_TryDelta_Apply_WildWest_ReturnsDeltaRequest(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	result, cmd := m.Update(keyMsg("M"))
	_ = result.(Model)
	if cmd == nil {
		t.Fatal("M key in wild-west with branch should return cmd")
	}

	msg := cmd()
	req, ok := msg.(deltaRequestMsg)
	if !ok {
		t.Fatalf("expected deltaRequestMsg, got %T", msg)
	}
	if req.action != deltaApply {
		t.Errorf("action = %v, want deltaApply", req.action)
	}
	if !strings.Contains(req.label, "Apply claim to main") {
		t.Errorf("label should mention 'Apply claim to main', got: %q", req.label)
	}
}

func TestDetail_TryDelta_Discard_ReturnsDeltaRequest(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	result, cmd := m.Update(keyMsg("b"))
	_ = result.(Model)
	if cmd == nil {
		t.Fatal("b key with branch should return cmd")
	}

	msg := cmd()
	req, ok := msg.(deltaRequestMsg)
	if !ok {
		t.Fatalf("expected deltaRequestMsg, got %T", msg)
	}
	if req.action != deltaDiscard {
		t.Errorf("action = %v, want deltaDiscard", req.action)
	}
	if !strings.Contains(req.label, "Discard claim") {
		t.Errorf("label should mention 'Discard claim', got: %q", req.label)
	}
	if !strings.Contains(req.label, "Reverts to open") {
		t.Errorf("label should mention 'Reverts to open', got: %q", req.label)
	}
}

func TestDetail_SetData_StoresMainStatus(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")
	m.detail.setData(detailDataMsg{
		item: &commons.WantedItem{
			ID:     "w-abc123",
			Title:  "Test",
			Status: "claimed",
		},
		branch:     "wl/test-rig/w-abc123",
		mainStatus: "open",
	})

	if m.detail.mainStatus != "open" {
		t.Errorf("mainStatus = %q, want %q", m.detail.mainStatus, "open")
	}
	if m.detail.branch != "wl/test-rig/w-abc123" {
		t.Errorf("branch = %q, want %q", m.detail.branch, "wl/test-rig/w-abc123")
	}
}

func TestDetail_SetData_StoresPRURL(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")
	m.detail.setData(detailDataMsg{
		item: &commons.WantedItem{
			ID:     "w-abc123",
			Title:  "Test",
			Status: "claimed",
		},
		branch:     "wl/test-rig/w-abc123",
		mainStatus: "open",
		prURL:      "https://github.com/org/repo/pull/42",
	})

	if m.detail.prURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("prURL = %q, want %q", m.detail.prURL, "https://github.com/org/repo/pull/42")
	}

	// View should show the PR line.
	v := m.View()
	if !strings.Contains(v, "PR:") {
		t.Errorf("view should contain 'PR:' line, got:\n%s", v)
	}
}
