package tui

import (
	"fmt"
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

func TestRootModel_DelegatesToBrowse(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
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
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
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
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
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

func TestDetail_DoneKey_OpensDoneForm(t *testing.T) {
	// Item is "claimed" by me → done is valid, opens form.
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")

	result, _ := m.Update(keyMsg("d"))
	m2 := result.(Model)

	if m2.detail.doneForm == nil {
		t.Fatal("'d' key should open done form")
	}
	if !m2.detail.doneForm.active {
		t.Error("done form should be active")
	}

	// View should contain done form elements.
	v := m2.View()
	if !strings.Contains(v, "Done:") {
		t.Errorf("view should contain 'Done:', got:\n%s", v)
	}
}

func TestDetail_AcceptKey_OpensAcceptForm(t *testing.T) {
	// Item is "in_review", posted by me, claimed by other.
	m := newDetailForTest("in_review", "test-rig", "other-rig", "wild-west")

	result, _ := m.Update(keyMsg("a"))
	m2 := result.(Model)

	if m2.detail.acceptForm == nil {
		t.Fatal("'a' key should open accept form")
	}
	if !m2.detail.acceptForm.active {
		t.Error("accept form should be active")
	}

	// View should contain accept form elements.
	v := m2.View()
	if !strings.Contains(v, "Accept:") {
		t.Errorf("view should contain 'Accept:', got:\n%s", v)
	}
}

func TestDetail_ActionResultMsg_WildWest_AppliesDetail(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "wild-west")
	m.detail.executing = true
	m.detail.executingLabel = "Claiming..."

	// Simulate successful wild-west result with detail from SDK.
	result, cmd := m.Update(actionResultMsg{
		result: &sdk.MutationResult{
			Detail: &sdk.DetailResult{
				Item: &commons.WantedItem{
					ID:        "w-abc123",
					Title:     "Test Item",
					Status:    "claimed",
					PostedBy:  "other-rig",
					ClaimedBy: "test-rig",
				},
			},
		},
	})
	m2 := result.(Model)
	if m2.detail.executing {
		t.Error("executing should be false after result")
	}
	if m2.detail.item.Status != "claimed" {
		t.Errorf("item status = %q, want %q", m2.detail.item.Status, "claimed")
	}
	// SDK provides detail directly — no re-fetch needed.
	if cmd != nil {
		t.Error("should not return cmd when SDK provides detail")
	}
}

func TestDetail_ActionResultMsg_PRMode_AppliesBranchDetail(t *testing.T) {
	m := newDetailForTest("open", "other-rig", "", "pr")
	m.detail.executing = true
	m.detail.executingLabel = "Claiming..."

	// Simulate successful PR result with detail from SDK.
	result, cmd := m.Update(actionResultMsg{
		result: &sdk.MutationResult{
			Detail: &sdk.DetailResult{
				Item: &commons.WantedItem{
					ID:        "w-abc123",
					Title:     "Test Item",
					Status:    "claimed",
					PostedBy:  "other-rig",
					ClaimedBy: "test-rig",
				},
			},
			Branch: "wl/test-rig/w-abc123",
		},
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
	// Open item, posted by someone else — I can claim, but not delete/close/reject.
	m := newDetailForTest("open", "other-rig", "", "wild-west")
	hints := m.detail.actionHints()

	if !strings.Contains(hints, "c:claim") {
		t.Errorf("hints should contain 'c:claim', got: %q", hints)
	}
	if strings.Contains(hints, "D:delete") {
		t.Errorf("hints should NOT contain 'D:delete' for non-poster, got: %q", hints)
	}

	// Poster should see both claim and delete.
	m2 := newDetailForTest("open", "test-rig", "", "wild-west")
	hints2 := m2.detail.actionHints()
	if !strings.Contains(hints2, "D:delete") {
		t.Errorf("poster hints should contain 'D:delete', got: %q", hints2)
	}
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

func TestRootModel_MeKey_NavigatesToMe(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	// Press 'm' to navigate to me dashboard.
	result, cmd := m.Update(keyMsg("m"))
	m2 := result.(Model)

	if cmd == nil {
		t.Fatal("after 'm': expected a cmd, got nil")
	}
	// Execute the cmd to get the navigateMsg.
	msg := cmd()
	nav, ok := msg.(navigateMsg)
	if !ok {
		t.Fatalf("expected navigateMsg, got %T", msg)
	}
	if nav.view != viewMe {
		t.Errorf("expected viewMe, got %d", nav.view)
	}

	// Feed the navigate msg back in.
	result, cmd = m2.Update(nav)
	m3 := result.(Model)
	if m3.active != viewMe {
		t.Errorf("active = %d, want viewMe", m3.active)
	}
	if !m3.me.loading {
		t.Error("me should be loading after navigation")
	}
	if cmd == nil {
		t.Error("expected fetchMe cmd")
	}
}

func TestRootModel_MeDataMsg_SetsData(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.active = viewMe
	m.me.loading = true
	m.width = 80
	m.height = 24

	data := &commons.DashboardData{
		Claimed: []commons.WantedSummary{
			{ID: "w-123", Title: "Test", Status: "claimed", Priority: 1},
		},
	}
	result, _ := m.Update(meDataMsg{data: data})
	m2 := result.(Model)

	if m2.me.loading {
		t.Error("me should not be loading after data msg")
	}
	if m2.me.data == nil {
		t.Fatal("me.data should be set")
	}
	if len(m2.me.data.Claimed) != 1 {
		t.Errorf("claimed items = %d, want 1", len(m2.me.data.Claimed))
	}
}

func TestMe_EscReturns(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.active = viewMe
	m.me.loading = false
	m.me.data = &commons.DashboardData{}
	m.width = 80
	m.height = 24

	// Press esc to go back.
	result, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	_ = result.(Model)

	if cmd == nil {
		t.Fatal("expected cmd from esc, got nil")
	}
	msg := cmd()
	nav, ok := msg.(navigateMsg)
	if !ok {
		t.Fatalf("expected navigateMsg, got %T", msg)
	}
	if nav.view != viewBrowse {
		t.Errorf("expected viewBrowse, got %d", nav.view)
	}
}

func TestMe_EnterOpensDetail(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.active = viewMe
	m.width = 80
	m.height = 24
	m.me.loading = false
	m.me.data = &commons.DashboardData{
		Claimed: []commons.WantedSummary{
			{ID: "w-test1", Title: "Item 1", Status: "claimed", Priority: 1},
		},
	}
	m.me.cursor = 0

	result, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	_ = result.(Model)

	if cmd == nil {
		t.Fatal("expected cmd from enter, got nil")
	}
	msg := cmd()
	nav, ok := msg.(navigateMsg)
	if !ok {
		t.Fatalf("expected navigateMsg, got %T", msg)
	}
	if nav.view != viewDetail {
		t.Errorf("expected viewDetail, got %d", nav.view)
	}
	if nav.wantedID != "w-test1" {
		t.Errorf("wantedID = %q, want %q", nav.wantedID, "w-test1")
	}
}

func TestMe_View_ShowsSections(t *testing.T) {
	m := newMeModel()
	m.loading = false
	m.width = 80
	m.height = 24
	m.data = &commons.DashboardData{
		Claimed: []commons.WantedSummary{
			{ID: "w-1", Title: "Claimed item", Status: "claimed", Priority: 1, Project: "proj"},
		},
		InReview: []commons.WantedSummary{
			{ID: "w-2", Title: "Review item", Status: "in_review", Priority: 2, Project: "proj"},
		},
		Completed: []commons.WantedSummary{
			{ID: "w-3", Title: "Done item", Status: "completed", Priority: 2, Project: "proj"},
		},
	}

	v := m.view()
	if !strings.Contains(v, "My Dashboard") {
		t.Errorf("view should contain title, got:\n%s", v)
	}
	if !strings.Contains(v, "My Claimed Items") {
		t.Errorf("view should contain claimed section, got:\n%s", v)
	}
	if !strings.Contains(v, "Awaiting My Review") {
		t.Errorf("view should contain review section, got:\n%s", v)
	}
	if !strings.Contains(v, "Recent Completions") {
		t.Errorf("view should contain completions section, got:\n%s", v)
	}
}

func TestRootModel_ProjectFilter_RoundTrip(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	// Enter project mode via root Update.
	result, _ := m.Update(keyMsg("P"))
	m = result.(Model)
	if !m.browse.projectMode {
		t.Fatal("should be in project mode")
	}

	// Type "gastown" through root Update.
	for _, ch := range "gastown" {
		result, _ = m.Update(keyMsg(string(ch)))
		m = result.(Model)
	}
	if m.browse.project.Value() != "gastown" {
		t.Fatalf("project value through root = %q, want %q", m.browse.project.Value(), "gastown")
	}

	// Press Enter through root Update.
	result, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	m = result.(Model)
	if m.browse.projectMode {
		t.Error("project mode should be off")
	}
	if cmd == nil {
		t.Fatal("expected fetchBrowse cmd")
	}

	// Verify projectFilter stored value.
	if m.browse.projectFilter != "gastown" {
		t.Errorf("projectFilter = %q, want %q", m.browse.projectFilter, "gastown")
	}
	f := m.browse.filter(m.cfg.RigHandle)
	if f.Project != "gastown" {
		t.Errorf("filter Project = %q, want %q", f.Project, "gastown")
	}

	// Simulate data arriving.
	result, _ = m.Update(browseDataMsg{items: nil})
	m = result.(Model)

	// Cycle status through root — project should survive.
	result, cmd = m.Update(keyMsg("s"))
	m = result.(Model)
	if cmd == nil {
		t.Fatal("expected cmd after 's'")
	}
	if m.browse.projectFilter != "gastown" {
		t.Errorf("after status cycle, projectFilter = %q, want %q", m.browse.projectFilter, "gastown")
	}
	f = m.browse.filter(m.cfg.RigHandle)
	if f.Project != "gastown" {
		t.Errorf("after status cycle, filter Project = %q, want %q", f.Project, "gastown")
	}

	// View should show "Project: gastown".
	v := m.View()
	if !strings.Contains(v, "gastown") {
		t.Errorf("view should show 'gastown' in filter bar, got:\n%s", v)
	}
}

func TestDelta_RequestMsg_ShowsConfirmation(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	result, cmd := m.Update(deltaRequestMsg{
		action: deltaApply,
		label:  "Apply claim to main? Pushes to origin. [y/n]",
	})
	m2 := result.(Model)

	if m2.detail.deltaConfirm == nil {
		t.Fatal("deltaRequestMsg should set deltaConfirm")
	}
	if m2.detail.deltaConfirm.action != deltaApply {
		t.Errorf("deltaConfirm action = %v, want deltaApply", m2.detail.deltaConfirm.action)
	}
	if cmd != nil {
		t.Error("deltaRequestMsg should not return a cmd")
	}

	// View should show the confirmation.
	v := m2.View()
	if !strings.Contains(v, "Apply claim to main") {
		t.Errorf("view should contain confirmation text, got:\n%s", v)
	}
}

func TestDelta_ConfirmYes_ReturnsDeltaConfirmed(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.deltaConfirm = &deltaConfirmAction{
		action: deltaApply,
		label:  "Apply claim to main? [y/n]",
	}

	result, cmd := m.Update(keyMsg("y"))
	m2 := result.(Model)

	if m2.detail.deltaConfirm != nil {
		t.Error("after 'y': deltaConfirm should be cleared")
	}
	if cmd == nil {
		t.Fatal("after 'y': expected cmd, got nil")
	}

	msg := cmd()
	confirmed, ok := msg.(deltaConfirmedMsg)
	if !ok {
		t.Fatalf("expected deltaConfirmedMsg, got %T", msg)
	}
	if confirmed.action != deltaApply {
		t.Errorf("confirmed action = %v, want deltaApply", confirmed.action)
	}
}

func TestDelta_ConfirmCancel_ClearsPrompt(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.deltaConfirm = &deltaConfirmAction{
		action: deltaApply,
		label:  "Apply claim to main? [y/n]",
	}

	result, cmd := m.Update(keyMsg("n"))
	m2 := result.(Model)
	if m2.detail.deltaConfirm != nil {
		t.Error("after 'n': deltaConfirm should be nil")
	}
	if cmd != nil {
		t.Error("after 'n': should have no cmd")
	}
}

func TestDelta_ConfirmedMsg_SetsExecuting(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	result, cmd := m.Update(deltaConfirmedMsg{action: deltaApply})
	m2 := result.(Model)

	if !m2.detail.executing {
		t.Error("executing should be true after deltaConfirmedMsg")
	}
	if m2.detail.executingLabel != "Applying..." {
		t.Errorf("executingLabel = %q, want %q", m2.detail.executingLabel, "Applying...")
	}
	if cmd == nil {
		t.Error("should return executeDelta cmd")
	}
}

func TestDelta_ConfirmedMsg_Discard_SetsExecuting(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"

	result, _ := m.Update(deltaConfirmedMsg{action: deltaDiscard})
	m2 := result.(Model)

	if !m2.detail.executing {
		t.Error("executing should be true after deltaConfirmedMsg")
	}
	if m2.detail.executingLabel != "Discarding..." {
		t.Errorf("executingLabel = %q, want %q", m2.detail.executingLabel, "Discarding...")
	}
}

func TestDelta_ResultMsg_Error(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.executing = true
	m.detail.executingLabel = "Applying..."

	result, cmd := m.Update(deltaResultMsg{err: fmt.Errorf("merge conflict")})
	m2 := result.(Model)

	if m2.detail.executing {
		t.Error("executing should be false after error result")
	}
	if !strings.Contains(m2.detail.result, "merge conflict") {
		t.Errorf("result should contain error, got: %q", m2.detail.result)
	}
	if cmd != nil {
		t.Error("error result should not return cmd")
	}
}

func TestDelta_ResultMsg_Applied_RefetchesDetail(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.executing = true
	m.detail.executingLabel = "Applying..."

	result, cmd := m.Update(deltaResultMsg{hint: "applied"})
	m2 := result.(Model)

	if m2.detail.executing {
		t.Error("executing should be false after result")
	}
	// Apply should re-fetch detail (branch is gone, item on main).
	if cmd == nil {
		t.Error("apply result should return fetchDetail cmd")
	}
	if m2.active != viewDetail {
		t.Errorf("should stay on detail view, got %d", m2.active)
	}
}

func TestDelta_ResultMsg_Discarded_NavigatesToBrowse(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.executing = true
	m.detail.executingLabel = "Discarding..."

	result, cmd := m.Update(deltaResultMsg{hint: "discarded"})
	m2 := result.(Model)

	if m2.detail.executing {
		t.Error("executing should be false after result")
	}
	// Discard should navigate back to browse.
	if m2.active != viewBrowse {
		t.Errorf("should navigate to browse, got %d", m2.active)
	}
	if cmd == nil {
		t.Error("discard should return fetchBrowse cmd")
	}
}

func TestMe_View_Hints(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.active = viewMe
	m.me.loading = false
	m.me.data = &commons.DashboardData{}
	m.width = 80
	m.height = 24

	v := m.View()
	if !strings.Contains(v, "esc: back") {
		t.Errorf("me view hints should contain 'esc: back', got:\n%s", v)
	}
}

func TestRootModel_SettingsKey_NavigatesToSettings(t *testing.T) {
	m := New(Config{
		RigHandle: "test",
		Upstream:  "test/db",
		Mode:      "wild-west",
		Signing:   false,
	})
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	// Press 'S' to navigate to settings.
	result, cmd := m.Update(keyMsg("S"))
	_ = result.(Model)

	if cmd == nil {
		t.Fatal("after 'S': expected a cmd, got nil")
	}

	msg := cmd()
	nav, ok := msg.(navigateMsg)
	if !ok {
		t.Fatalf("expected navigateMsg, got %T", msg)
	}
	if nav.view != viewSettings {
		t.Errorf("expected viewSettings, got %d", nav.view)
	}

	// Feed the navigate msg back in.
	result, _ = m.Update(nav)
	m2 := result.(Model)
	if m2.active != viewSettings {
		t.Errorf("active = %d, want viewSettings", m2.active)
	}

	// View should show settings content.
	v := m2.View()
	if !strings.Contains(v, "Settings") {
		t.Errorf("view should contain 'Settings', got:\n%s", v)
	}
	if !strings.Contains(v, "j/k: select") {
		t.Errorf("hints should contain 'j/k: select', got:\n%s", v)
	}
}

func TestRootModel_SettingsFromMe(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db", Mode: "wild-west"})
	m.active = viewMe
	m.me.loading = false
	m.me.data = &commons.DashboardData{}
	m.width = 80
	m.height = 24

	result, cmd := m.Update(keyMsg("S"))
	_ = result.(Model)

	if cmd == nil {
		t.Fatal("after 'S' from me: expected a cmd, got nil")
	}
	msg := cmd()
	nav, ok := msg.(navigateMsg)
	if !ok {
		t.Fatalf("expected navigateMsg, got %T", msg)
	}
	if nav.view != viewSettings {
		t.Errorf("expected viewSettings, got %d", nav.view)
	}
}

func TestRootModel_SettingsSavedMsg_UpdatesConfig(t *testing.T) {
	m := New(Config{
		RigHandle: "test",
		Upstream:  "test/db",
		Mode:      "wild-west",
		Signing:   false,
	})
	m.active = viewSettings
	m.width = 80
	m.height = 24

	// Simulate a settings save.
	result, _ := m.Update(settingsSavedMsg{mode: "pr", signing: true})
	m2 := result.(Model)

	if m2.cfg.Mode != "pr" {
		t.Errorf("cfg.Mode = %q, want %q", m2.cfg.Mode, "pr")
	}
	if !m2.cfg.Signing {
		t.Error("cfg.Signing should be true")
	}
	if m2.detail.mode != "pr" {
		t.Errorf("detail.mode = %q, want %q", m2.detail.mode, "pr")
	}
	if !strings.Contains(m2.settings.result, "Saved") {
		t.Errorf("settings.result should contain 'Saved', got %q", m2.settings.result)
	}
}

func TestRootModel_SettingsSavedMsg_Error(t *testing.T) {
	m := New(Config{
		RigHandle: "test",
		Upstream:  "test/db",
		Mode:      "wild-west",
	})
	m.active = viewSettings
	m.width = 80
	m.height = 24

	result, _ := m.Update(settingsSavedMsg{mode: "pr", signing: true, err: fmt.Errorf("disk full")})
	m2 := result.(Model)

	// Config should NOT be updated on error.
	if m2.cfg.Mode != "wild-west" {
		t.Errorf("cfg.Mode = %q, want %q (unchanged)", m2.cfg.Mode, "wild-west")
	}
	if !strings.Contains(m2.settings.result, "disk full") {
		t.Errorf("settings.result should contain error, got %q", m2.settings.result)
	}
}

func TestSettings_BrowseHints_ShowsSettingsKey(t *testing.T) {
	m := New(Config{RigHandle: "test", Upstream: "test/db"})
	m.browse.loading = false
	m.width = 80
	m.height = 24
	m.browse.setSize(80, 23)

	v := m.View()
	if !strings.Contains(v, "S: settings") {
		t.Errorf("browse hints should contain 'S: settings', got:\n%s", v)
	}
}

// --- Done/Accept/Submit integration tests ---

func TestDetail_DoneSubmitMsg_SetsExecuting(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")
	m.detail.doneForm = newDoneForm()

	result, cmd := m.Update(doneSubmitMsg{evidence: "https://example.com/pr/1"})
	m2 := result.(Model)

	if m2.detail.doneForm != nil {
		t.Error("doneForm should be cleared after doneSubmitMsg")
	}
	if !m2.detail.executing {
		t.Error("executing should be true")
	}
	if m2.detail.executingLabel != "Submitting..." {
		t.Errorf("executingLabel = %q, want %q", m2.detail.executingLabel, "Submitting...")
	}
	if cmd == nil {
		t.Error("should return executeDoneMutation cmd")
	}
}

func TestDetail_AcceptSubmitMsg_SetsExecuting(t *testing.T) {
	m := newDetailForTest("in_review", "test-rig", "other-rig", "wild-west")
	m.detail.completion = &commons.CompletionRecord{ID: "c-test"}
	m.detail.acceptForm = newAcceptForm()

	result, cmd := m.Update(acceptSubmitMsg{
		quality:     4,
		reliability: 4,
		severity:    "leaf",
		skills:      []string{"go"},
		message:     "good work",
	})
	m2 := result.(Model)

	if m2.detail.acceptForm != nil {
		t.Error("acceptForm should be cleared after acceptSubmitMsg")
	}
	if !m2.detail.executing {
		t.Error("executing should be true")
	}
	if m2.detail.executingLabel != "Accepting..." {
		t.Errorf("executingLabel = %q, want %q", m2.detail.executingLabel, "Accepting...")
	}
	if cmd == nil {
		t.Error("should return executeAcceptMutation cmd")
	}
}

func TestDetail_DoneFormEsc_ClearsForm(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")

	// Open done form.
	result, _ := m.Update(keyMsg("d"))
	m2 := result.(Model)
	if m2.detail.doneForm == nil {
		t.Fatal("done form should be open")
	}

	// Press esc to cancel.
	result, _ = m2.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	m3 := result.(Model)
	if m3.detail.doneForm != nil {
		t.Error("esc should clear done form")
	}
}

func TestDetail_AcceptFormEsc_ClearsForm(t *testing.T) {
	m := newDetailForTest("in_review", "test-rig", "other-rig", "wild-west")

	// Open accept form.
	result, _ := m.Update(keyMsg("a"))
	m2 := result.(Model)
	if m2.detail.acceptForm == nil {
		t.Fatal("accept form should be open")
	}

	// Press esc to cancel.
	result, _ = m2.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	m3 := result.(Model)
	if m3.detail.acceptForm != nil {
		t.Error("esc should clear accept form")
	}
}

func TestDetail_SubmitDiffMsg_SetsDiff(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.submit = newSubmitModel(m.detail.item, m.detail.branch, m.detail.mainStatus, 80, 22)

	result, _ := m.Update(submitDiffMsg{diff: "diff content here"})
	m2 := result.(Model)

	if m2.detail.submit == nil {
		t.Fatal("submit should still be active")
	}
	if !m2.detail.submit.diffLoaded {
		t.Error("diff should be loaded")
	}
	if m2.detail.submit.diff != "diff content here" {
		t.Errorf("diff = %q, want %q", m2.detail.submit.diff, "diff content here")
	}
}

func TestDetail_SubmitConfirmMsg_CreatesExecuting(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.branch = "wl/test-rig/w-abc123"
	m.detail.mainStatus = "open"
	m.detail.submit = newSubmitModel(m.detail.item, m.detail.branch, m.detail.mainStatus, 80, 22)

	result, cmd := m.Update(submitConfirmMsg{})
	m2 := result.(Model)

	if m2.detail.submit != nil {
		t.Error("submit should be cleared after confirm")
	}
	if !m2.detail.executing {
		t.Error("executing should be true")
	}
	if m2.detail.executingLabel != "Creating PR..." {
		t.Errorf("executingLabel = %q, want %q", m2.detail.executingLabel, "Creating PR...")
	}
	if cmd == nil {
		t.Error("should return createPR cmd")
	}
}

func TestDetail_SubmitResultMsg_Success(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.executing = true
	m.detail.executingLabel = "Creating PR..."

	result, _ := m.Update(submitResultMsg{prURL: "https://github.com/org/repo/pull/42"})
	m2 := result.(Model)

	if m2.detail.executing {
		t.Error("executing should be false")
	}
	if !strings.Contains(m2.detail.result, "PR created") {
		t.Errorf("result should contain 'PR created', got: %q", m2.detail.result)
	}
	if !strings.Contains(m2.detail.result, "https://github.com/org/repo/pull/42") {
		t.Errorf("result should contain PR URL, got: %q", m2.detail.result)
	}
	// prURL should be stored so M key won't offer submit again.
	if m2.detail.prURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("prURL should be stored, got: %q", m2.detail.prURL)
	}
}

func TestDetail_SubmitResultMsg_Error(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "pr")
	m.detail.executing = true

	result, _ := m.Update(submitResultMsg{err: fmt.Errorf("gh not found")})
	m2 := result.(Model)

	if m2.detail.executing {
		t.Error("executing should be false")
	}
	if !strings.Contains(m2.detail.result, "gh not found") {
		t.Errorf("result should contain error, got: %q", m2.detail.result)
	}
}

func TestDetail_SubmitOpenedMsg_DispatchesFetchDiff(t *testing.T) {
	diffCalled := false
	client := sdk.New(sdk.ClientConfig{
		RigHandle: "test-rig",
		Mode:      "pr",
		LoadDiff: func(_ string) (string, error) {
			diffCalled = true
			return "mock diff", nil
		},
	})
	m := New(Config{
		Client:    client,
		RigHandle: "test-rig",
		Upstream:  "test/db",
		Mode:      "pr",
	})
	m.active = viewDetail

	result, cmd := m.Update(submitOpenedMsg{branch: "wl/test-rig/w-abc123"})
	_ = result.(Model)

	if cmd == nil {
		t.Fatal("submitOpenedMsg should return fetchDiff cmd")
	}

	// Execute the cmd.
	msg := cmd()
	diffMsg, ok := msg.(submitDiffMsg)
	if !ok {
		t.Fatalf("expected submitDiffMsg, got %T", msg)
	}
	if !diffCalled {
		t.Error("LoadDiff callback should have been called")
	}
	if diffMsg.diff != "mock diff" {
		t.Errorf("diff = %q, want %q", diffMsg.diff, "mock diff")
	}
}

func TestDetail_DoneKey_InvalidTransition_ShowsError(t *testing.T) {
	// Item is "open", done requires "claimed".
	m := newDetailForTest("open", "other-rig", "", "wild-west")

	result, _ := m.Update(keyMsg("d"))
	m2 := result.(Model)

	if m2.detail.doneForm != nil {
		t.Error("done form should not open for invalid transition")
	}
	if !strings.Contains(m2.detail.result, "cannot") {
		t.Errorf("result should contain error, got: %q", m2.detail.result)
	}
}

func TestDetail_AcceptKey_PermissionDenied(t *testing.T) {
	// Item is "in_review", but I'm the claimant (not poster) — self-accept blocked.
	m := newDetailForTest("in_review", "other-poster", "test-rig", "wild-west")

	result, _ := m.Update(keyMsg("a"))
	m2 := result.(Model)

	if m2.detail.acceptForm != nil {
		t.Error("accept form should not open when self-accepting")
	}
	if !strings.Contains(m2.detail.result, "permission denied") {
		t.Errorf("result should contain permission denied, got: %q", m2.detail.result)
	}
}

func TestDetail_SetData_ClearsForms(t *testing.T) {
	m := newDetailForTest("claimed", "other-rig", "test-rig", "wild-west")
	m.detail.doneForm = newDoneForm()
	m.detail.acceptForm = newAcceptForm()
	m.detail.submit = newSubmitModel(m.detail.item, "wl/test-rig/w-abc123", "open", 80, 22)

	m.detail.setData(detailDataMsg{
		item: &commons.WantedItem{
			ID:     "w-abc123",
			Title:  "Test Item",
			Status: "in_review",
		},
	})

	if m.detail.doneForm != nil {
		t.Error("setData should clear doneForm")
	}
	if m.detail.acceptForm != nil {
		t.Error("setData should clear acceptForm")
	}
	if m.detail.submit != nil {
		t.Error("setData should clear submit")
	}
}
