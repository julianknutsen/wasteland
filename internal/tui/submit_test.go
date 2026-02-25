package tui

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestSubmitModel_New(t *testing.T) {
	item := &commons.WantedItem{
		ID:    "w-abc123",
		Title: "Fix the login bug",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)
	if sm.item != item {
		t.Error("item should be set")
	}
	if sm.branch != "wl/myrig/w-abc123" {
		t.Errorf("branch = %q, want %q", sm.branch, "wl/myrig/w-abc123")
	}
	if sm.showDiff {
		t.Error("diff should be collapsed by default")
	}
	if sm.diffLoaded {
		t.Error("diff should not be loaded initially")
	}
}

func TestSubmitModel_View_ContainsElements(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)
	v := sm.view()

	if !strings.Contains(v, "Submit PR") {
		t.Errorf("view should contain 'Submit PR', got:\n%s", v)
	}
	if !strings.Contains(v, "w-abc123") {
		t.Errorf("view should contain wanted ID, got:\n%s", v)
	}
	if !strings.Contains(v, "wl/myrig/w-abc123") {
		t.Errorf("view should contain branch, got:\n%s", v)
	}
	if !strings.Contains(v, "press tab to show diff") {
		t.Errorf("view should show diff toggle hint, got:\n%s", v)
	}
	if !strings.Contains(v, "enter: create PR") {
		t.Errorf("view should contain 'enter: create PR', got:\n%s", v)
	}
}

func TestSubmitModel_TabTogglesDiff(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)

	if sm.showDiff {
		t.Fatal("diff should start collapsed")
	}

	// Press tab to expand.
	result, _ := sm.update(keyMsg("tab"))
	sm = result
	if !sm.showDiff {
		t.Error("after tab: diff should be expanded")
	}

	// Press tab again to collapse.
	result, _ = sm.update(keyMsg("tab"))
	sm = result
	if sm.showDiff {
		t.Error("after second tab: diff should be collapsed")
	}
}

func TestSubmitModel_SetDiff(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)
	sm.showDiff = true

	sm.setDiff(submitDiffMsg{diff: "some diff content"})

	if !sm.diffLoaded {
		t.Error("diffLoaded should be true")
	}
	if sm.diff != "some diff content" {
		t.Errorf("diff = %q, want %q", sm.diff, "some diff content")
	}

	v := sm.view()
	if !strings.Contains(v, "some diff content") {
		t.Errorf("view should contain diff content, got:\n%s", v)
	}
}

func TestSubmitModel_SetDiff_Error(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)
	sm.showDiff = true

	sm.setDiff(submitDiffMsg{diff: "", err: errForTest("diff failed")})

	if !sm.diffLoaded {
		t.Error("diffLoaded should be true")
	}

	v := sm.view()
	if !strings.Contains(v, "Diff error") {
		t.Errorf("view should contain diff error, got:\n%s", v)
	}
}

func TestSubmitModel_Escape_Returns(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)

	result, cmd := sm.update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	if result != nil {
		t.Error("esc should cancel submit (return nil)")
	}
	if cmd != nil {
		t.Error("esc should not return a cmd")
	}
}

func TestSubmitModel_Enter_ReturnsConfirmMsg(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)

	result, cmd := sm.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if result == nil {
		t.Fatal("enter should not cancel submit")
	}
	if cmd == nil {
		t.Fatal("enter should return submitConfirmMsg cmd")
	}

	msg := cmd()
	if _, ok := msg.(submitConfirmMsg); !ok {
		t.Fatalf("expected submitConfirmMsg, got %T", msg)
	}
}

func TestSubmitModel_DiffLoadingState(t *testing.T) {
	item := &commons.WantedItem{
		ID:     "w-abc123",
		Title:  "Fix the login bug",
		Status: "claimed",
	}
	sm := newSubmitModel(item, "wl/myrig/w-abc123", "open", 80, 24)
	sm.showDiff = true
	sm.refreshContent()

	v := sm.view()
	if !strings.Contains(v, "Loading diff") {
		t.Errorf("view should show loading state when diff not loaded, got:\n%s", v)
	}
}

type errForTest string

func (e errForTest) Error() string { return string(e) }
