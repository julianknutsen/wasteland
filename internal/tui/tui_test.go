package tui

import (
	"fmt"
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestRootModel_DelegatesToBrowse(t *testing.T) {
	m := New(Config{DBDir: "/tmp/fake", RigHandle: "test", Upstream: "test/db"})
	// Simulate initial load completing.
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	// Press 's' to cycle status.
	msg := bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune("s")}
	result, cmd := m.Update(msg)
	m2 := result.(Model)

	if m2.browse.statusIdx != 1 {
		t.Errorf("after 's': statusIdx = %d, want 1", m2.browse.statusIdx)
	}
	if !m2.browse.loading {
		t.Error("after 's': browse should be loading")
	}
	if cmd == nil {
		t.Error("after 's': expected a cmd, got nil")
	}

	// View should show "Status: claimed".
	v := m2.View()
	if !strings.Contains(v, "Status: claimed") {
		t.Errorf("view should show 'Status: claimed', got:\n%s", v)
	}
}

func TestRootModel_SearchKey(t *testing.T) {
	m := New(Config{DBDir: "/tmp/fake", RigHandle: "test", Upstream: "test/db"})
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	// Press '/' to enter search mode.
	msg := bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune("/")}
	result, _ := m.Update(msg)
	m2 := result.(Model)

	if !m2.browse.searchMode {
		t.Error("after '/': browse should be in search mode")
	}

	v := m2.View()
	if !strings.Contains(v, "search") {
		t.Errorf("view should contain search placeholder, got:\n%s", v)
	}
}

func TestRootModel_TypeKey(t *testing.T) {
	m := New(Config{DBDir: "/tmp/fake", RigHandle: "test", Upstream: "test/db"})
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	// Press 't' to cycle type.
	msg := bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune("t")}
	result, cmd := m.Update(msg)
	m2 := result.(Model)

	if m2.browse.typeIdx != 1 {
		t.Errorf("after 't': typeIdx = %d, want 1", m2.browse.typeIdx)
	}
	if cmd == nil {
		t.Error("after 't': expected a cmd, got nil")
	}

	v := m2.View()
	if !strings.Contains(v, "Type: feature") {
		t.Errorf("view should show 'Type: feature', got:\n%s", v)
	}
}

// newDetailForTest creates a detail model with a loaded item for mutation testing.
func newDetailForTest(status, postedBy, claimedBy, mode string) Model {
	m := New(Config{
		DBDir:     "/tmp/fake",
		RigHandle: "test-rig",
		Upstream:  "test/db",
		Mode:      mode,
	})
	m.active = viewDetail
	m.width = 80
	m.height = 24
	m.detail.setSize(80, 23)
	m.detail.setData(detailDataMsg{
		item: &commons.WantedItem{
			ID:        "w-abc123",
			Title:     "Test Item",
			Status:    status,
			PostedBy:  postedBy,
			ClaimedBy: claimedBy,
		},
	})
	return m
}

func TestDetail_ClaimKeyWildWest_ShowsConfirmation(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")

	// Press 'c' to claim.
	result, cmd := m.Update(keyMsg("c"))
	m2 := result.(Model)

	// Should return an actionRequestMsg cmd (not nil).
	if cmd == nil {
		t.Fatal("expected cmd from 'c' key, got nil")
	}

	// Execute the cmd to get the actionRequestMsg.
	msg := cmd()
	req, ok := msg.(actionRequestMsg)
	if !ok {
		t.Fatalf("expected actionRequestMsg, got %T", msg)
	}
	if req.transition != commons.TransitionClaim {
		t.Errorf("transition = %v, want TransitionClaim", req.transition)
	}

	// Feed the actionRequestMsg into root Update — wild-west should set confirming.
	result, _ = m2.Update(req)
	m3 := result.(Model)
	if m3.detail.confirming == nil {
		t.Fatal("wild-west mode should show confirmation prompt")
	}
	if m3.detail.confirming.transition != commons.TransitionClaim {
		t.Errorf("confirming transition = %v, want TransitionClaim", m3.detail.confirming.transition)
	}

	// View should contain confirmation text.
	v := m3.View()
	if !strings.Contains(v, "Claim w-abc123?") {
		t.Errorf("view should contain 'Claim w-abc123?', got:\n%s", v)
	}
	if !strings.Contains(v, "[y/n]") {
		t.Errorf("view should contain '[y/n]', got:\n%s", v)
	}
}

func TestDetail_ConfirmCancel_ClearsPrompt(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")

	// Set up confirming state directly.
	m.detail.confirming = &confirmAction{
		transition: commons.TransitionClaim,
		label:      "Claim w-abc123?",
	}

	// Press 'n' to cancel.
	result, cmd := m.Update(keyMsg("n"))
	m2 := result.(Model)
	if m2.detail.confirming != nil {
		t.Error("after 'n': confirming should be nil")
	}
	if cmd != nil {
		t.Error("after 'n': should have no cmd")
	}
}

func TestDetail_ConfirmEsc_ClearsPrompt(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")

	m.detail.confirming = &confirmAction{
		transition: commons.TransitionClaim,
		label:      "Claim w-abc123?",
	}

	// Press esc to cancel.
	result, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	m2 := result.(Model)
	if m2.detail.confirming != nil {
		t.Error("after esc: confirming should be nil")
	}
	if cmd != nil {
		t.Error("after esc: should have no cmd")
	}
}

func TestDetail_ConfirmYes_ReturnsActionConfirmed(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")

	m.detail.confirming = &confirmAction{
		transition: commons.TransitionClaim,
		label:      "Claim w-abc123?",
	}

	// Press 'y' to confirm.
	result, cmd := m.Update(keyMsg("y"))
	m2 := result.(Model)
	if m2.detail.confirming != nil {
		t.Error("after 'y': confirming should be cleared")
	}
	if cmd == nil {
		t.Fatal("after 'y': expected cmd, got nil")
	}

	// Execute the cmd — should produce actionConfirmedMsg.
	msg := cmd()
	confirmed, ok := msg.(actionConfirmedMsg)
	if !ok {
		t.Fatalf("expected actionConfirmedMsg, got %T", msg)
	}
	if confirmed.transition != commons.TransitionClaim {
		t.Errorf("confirmed transition = %v, want TransitionClaim", confirmed.transition)
	}
}

func TestDetail_ClaimKeyPRMode_SkipsConfirmation(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")

	// Press 'c' → actionRequestMsg.
	_, cmd := m.Update(keyMsg("c"))
	if cmd == nil {
		t.Fatal("expected cmd from 'c' key, got nil")
	}
	msg := cmd()
	req, ok := msg.(actionRequestMsg)
	if !ok {
		t.Fatalf("expected actionRequestMsg, got %T", msg)
	}

	// Feed into root — PR mode should skip confirmation, go straight to executing.
	result, cmd := m.Update(req)
	m2 := result.(Model)
	if m2.detail.confirming != nil {
		t.Error("PR mode should NOT show confirmation prompt")
	}
	if !m2.detail.executing {
		t.Error("PR mode should set executing = true immediately")
	}
	// Executing label should be "Claiming..." not "Claim w-abc123?" (which looks like a confirmation).
	if m2.detail.executingLabel != "Claiming..." {
		t.Errorf("executingLabel = %q, want %q", m2.detail.executingLabel, "Claiming...")
	}
	if cmd == nil {
		t.Error("PR mode should return executeMutation cmd")
	}
}

func TestDetail_SetData_ClearsStaleResult(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")

	// Simulate a completed action leaving stale result.
	m.detail.result = styleSuccess.Render("Done")
	m.detail.refreshViewport()

	// View should show the stale result.
	v := m.View()
	if !strings.Contains(v, "Done") {
		t.Fatal("precondition: view should contain stale 'Done' result")
	}

	// Now simulate re-fetching detail (as happens after navigating back and re-entering).
	m.detail.setData(detailDataMsg{
		item: &commons.WantedItem{
			ID:        "w-abc123",
			Title:     "Test Item",
			Status:    "claimed",
			PostedBy:  "other-rig",
			ClaimedBy: "test-rig",
		},
	})

	// Result should be cleared, action hints should be visible.
	if m.detail.result != "" {
		t.Errorf("result should be cleared after setData, got: %q", m.detail.result)
	}
	v = m.View()
	if !strings.Contains(v, "u:unclaim") {
		t.Errorf("view should show action hints after setData, got:\n%s", v)
	}
}

func TestDetail_InvalidTransition_ShowsError(t *testing.T) {
	// Item is "open", so unclaim (requires "claimed") should fail.
	m := newDetailForTest("open", "test-rig", "", "wild-west")

	result, cmd := m.Update(keyMsg("u"))
	m2 := result.(Model)

	// Should not trigger confirmation — the transition is invalid.
	if m2.detail.confirming != nil {
		t.Error("invalid transition should not show confirmation")
	}
	if cmd != nil {
		t.Error("invalid transition should not return a cmd")
	}
	// The result message should indicate the error.
	if !strings.Contains(m2.detail.result, "cannot unclaim") {
		t.Errorf("result should contain error, got: %q", m2.detail.result)
	}
}

func TestDetail_DoneKey_ShowsCLIHint(t *testing.T) {
	// Item is "claimed" by me → done is valid, but needs CLI.
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")

	result, cmd := m.Update(keyMsg("d"))
	m2 := result.(Model)

	if cmd != nil {
		t.Error("'d' key should not return a cmd (needs CLI)")
	}
	if !strings.Contains(m2.detail.result, "wl done") {
		t.Errorf("result should contain CLI hint, got: %q", m2.detail.result)
	}
}

func TestDetail_AcceptKey_ShowsCLIHint(t *testing.T) {
	// Item is "in_review", posted by me, claimed by other.
	m := newDetailForTest("in_review", "test-rig", "other-rig", "wild-west")

	result, cmd := m.Update(keyMsg("a"))
	m2 := result.(Model)

	if cmd != nil {
		t.Error("'a' key should not return a cmd (needs CLI)")
	}
	if !strings.Contains(m2.detail.result, "wl accept") {
		t.Errorf("result should contain CLI hint, got: %q", m2.detail.result)
	}
}

func TestDetail_ActionResultMsg_WildWest_RefetchesDetail(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")
	m.detail.executing = true
	m.detail.executingLabel = "Claiming..."

	// Simulate successful wild-west result (no hint).
	result, cmd := m.Update(actionResultMsg{err: nil})
	m2 := result.(Model)
	if m2.detail.executing {
		t.Error("executing should be false after result")
	}
	// Wild-west clears result and re-fetches — the updated status is the feedback.
	if cmd == nil {
		t.Error("wild-west should return fetchDetail cmd to refresh")
	}
}

func TestDetail_ActionResultMsg_PRMode_AppliesBranchDetail(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")
	m.detail.executing = true
	m.detail.executingLabel = "Claiming..."

	// Simulate successful PR result with detail read from branch.
	branchDetail := detailDataMsg{
		item: &commons.WantedItem{
			ID:        "w-abc123",
			Title:     "Test Item",
			Status:    "claimed",
			PostedBy:  "other-rig",
			ClaimedBy: "test-rig",
		},
	}
	result, cmd := m.Update(actionResultMsg{
		hint:   "wl/test-rig/w-abc123",
		detail: &branchDetail,
	})
	m2 := result.(Model)
	if m2.detail.executing {
		t.Error("executing should be false after result")
	}
	// Detail should reflect the branch state.
	if m2.detail.item.Status != "claimed" {
		t.Errorf("item status = %q, want %q", m2.detail.item.Status, "claimed")
	}
	if !strings.Contains(m2.detail.result, "wl/test-rig/w-abc123") {
		t.Errorf("result should contain branch name, got: %q", m2.detail.result)
	}
	// View should show updated status and action hints.
	v := m2.View()
	if !strings.Contains(v, "claimed") {
		t.Errorf("view should show 'claimed' status, got:\n%s", v)
	}
	// Should NOT re-fetch from main.
	if cmd != nil {
		t.Error("PR mode should not return fetchDetail cmd")
	}
}

func TestDetail_ActionResultMsg_Error(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")
	m.detail.executing = true

	result, cmd := m.Update(actionResultMsg{err: fmt.Errorf("push failed")})
	m2 := result.(Model)
	if m2.detail.executing {
		t.Error("executing should be false after error result")
	}
	if !strings.Contains(m2.detail.result, "push failed") {
		t.Errorf("result should contain error, got: %q", m2.detail.result)
	}
	// Errors should NOT re-fetch.
	if cmd != nil {
		t.Error("error result should not return fetchDetail cmd")
	}
}

func TestDetail_PermissionDenied_Unclaim(t *testing.T) {
	// Item claimed by someone else, posted by someone else — can't unclaim.
	m := newDetailForTest("claimed", "other-poster", "other-claimer", "wild-west")

	result, cmd := m.Update(keyMsg("u"))
	m2 := result.(Model)

	if cmd != nil {
		t.Error("permission denied should not return a cmd")
	}
	if !strings.Contains(m2.detail.result, "permission denied") {
		t.Errorf("result should contain permission denied, got: %q", m2.detail.result)
	}
}

func TestDetail_ActionHints_PermissionFiltered(t *testing.T) {
	// Open item, posted by someone else — I can claim and delete, but not close/reject.
	m := newDetailForTest("open", "other-rig", "", "wild-west")
	hints := m.detail.actionHints()

	if !strings.Contains(hints, "c:claim") {
		t.Errorf("hints should contain 'c:claim', got: %q", hints)
	}
	if !strings.Contains(hints, "D:delete") {
		t.Errorf("hints should contain 'D:delete', got: %q", hints)
	}
	// close requires PostedBy == me, which is false here — but close is for in_review anyway.
	// For open items, only claim and delete are valid transitions.
}

func TestDetail_ExecutingState_IgnoresKeys(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")
	m.detail.executing = true

	// Keys should be ignored while executing.
	result, cmd := m.Update(keyMsg("c"))
	m2 := result.(Model)
	if !m2.detail.executing {
		t.Error("executing state should be preserved")
	}
	if cmd != nil {
		t.Error("should not return cmd while executing")
	}
}
