package tui

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
)

func keyMsg(s string) bubbletea.Msg {
	return bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune(s)}
}

func TestBrowseUpdate_StatusCycle(t *testing.T) {
	m := newBrowseModel()
	m.loading = false // simulate initial load done

	if m.statusIdx != 0 {
		t.Fatalf("initial statusIdx = %d, want 0", m.statusIdx)
	}

	m2, cmd := m.update(keyMsg("s"), Config{})
	if m2.statusIdx != 1 {
		t.Errorf("after 's': statusIdx = %d, want 1", m2.statusIdx)
	}
	if !m2.loading {
		t.Error("after 's': loading should be true")
	}
	if cmd == nil {
		t.Error("after 's': expected a cmd (fetchBrowse), got nil")
	}
}

func TestBrowseUpdate_TypeCycle(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	if m.typeIdx != 0 {
		t.Fatalf("initial typeIdx = %d, want 0", m.typeIdx)
	}

	m2, cmd := m.update(keyMsg("t"), Config{})
	if m2.typeIdx != 1 {
		t.Errorf("after 't': typeIdx = %d, want 1", m2.typeIdx)
	}
	if !m2.loading {
		t.Error("after 't': loading should be true")
	}
	if cmd == nil {
		t.Error("after 't': expected a cmd, got nil")
	}
}

func TestBrowseUpdate_PriorityCycle(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	if m.priorityIdx != 0 {
		t.Fatalf("initial priorityIdx = %d, want 0", m.priorityIdx)
	}

	m2, cmd := m.update(keyMsg("p"), Config{})
	if m2.priorityIdx != 1 {
		t.Errorf("after 'p': priorityIdx = %d, want 1", m2.priorityIdx)
	}
	if !m2.loading {
		t.Error("after 'p': loading should be true")
	}
	if cmd == nil {
		t.Error("after 'p': expected a cmd, got nil")
	}
}

func TestBrowseUpdate_SortCycle(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	if m.sortIdx != 0 {
		t.Fatalf("initial sortIdx = %d, want 0", m.sortIdx)
	}

	m2, cmd := m.update(keyMsg("o"), Config{})
	if m2.sortIdx != 1 {
		t.Errorf("after 'o': sortIdx = %d, want 1", m2.sortIdx)
	}
	if !m2.loading {
		t.Error("after 'o': loading should be true")
	}
	if cmd == nil {
		t.Error("after 'o': expected a cmd, got nil")
	}
}

func TestBrowseUpdate_MyItemsToggle(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	if m.myItems {
		t.Fatal("initial myItems should be false")
	}

	// statusIdx starts at 0 ("open").
	if m.statusIdx != 0 {
		t.Fatalf("initial statusIdx = %d, want 0", m.statusIdx)
	}

	m2, cmd := m.update(keyMsg("i"), Config{RigHandle: "test-rig"})
	if !m2.myItems {
		t.Error("after 'i': myItems should be true")
	}
	if !m2.loading {
		t.Error("after 'i': loading should be true")
	}
	if cmd == nil {
		t.Error("after 'i': expected a cmd, got nil")
	}
	// Toggling mine ON should reset status to "all" so items aren't hidden.
	allIdx := len(commons.ValidStatuses()) - 1
	if m2.statusIdx != allIdx {
		t.Errorf("after 'i': statusIdx = %d, want %d (all)", m2.statusIdx, allIdx)
	}

	// Toggle off — status stays where it is.
	m3, _ := m2.update(keyMsg("i"), Config{RigHandle: "test-rig"})
	if m3.myItems {
		t.Error("after second 'i': myItems should be false")
	}
}

func TestBrowseUpdate_ProjectMode(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	m2, _ := m.update(keyMsg("P"), Config{})
	if !m2.projectMode {
		t.Error("after 'P': projectMode should be true")
	}
	if !m2.project.Focused() {
		t.Error("after 'P': project input should be focused")
	}
}

func TestBrowseUpdate_ProjectFilter_AppliesOnEnter(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	cfg := Config{RigHandle: "test"}

	// Enter project mode.
	m, _ = m.update(keyMsg("P"), cfg)
	if !m.projectMode {
		t.Fatal("should be in project mode")
	}

	// Type "gastown" character by character.
	for _, ch := range "gastown" {
		m, _ = m.update(keyMsg(string(ch)), cfg)
	}
	if m.project.Value() != "gastown" {
		t.Fatalf("project value = %q, want %q", m.project.Value(), "gastown")
	}

	// Press Enter to apply.
	m, cmd := m.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter}, cfg)
	if m.projectMode {
		t.Error("project mode should be off after enter")
	}
	if cmd == nil {
		t.Fatal("expected fetchBrowse cmd after enter")
	}

	// Verify the filter has the project set.
	f := m.filter(cfg.RigHandle)
	if f.Project != "gastown" {
		t.Errorf("filter Project = %q, want %q", f.Project, "gastown")
	}
}

func TestBrowseUpdate_ProjectFilter_SurvivesStatusCycle(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	cfg := Config{RigHandle: "test"}

	// Set project via text input.
	m, _ = m.update(keyMsg("P"), cfg)
	for _, ch := range "gastown" {
		m, _ = m.update(keyMsg(string(ch)), cfg)
	}
	m, _ = m.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter}, cfg)

	// Simulate data arriving so loading=false.
	m.setData(browseDataMsg{items: nil})

	// Now cycle status — project should still be in the filter.
	m, cmd := m.update(keyMsg("s"), cfg)
	if cmd == nil {
		t.Fatal("expected fetchBrowse cmd after 's'")
	}

	f := m.filter(cfg.RigHandle)
	if f.Project != "gastown" {
		t.Errorf("after status cycle, filter Project = %q, want %q", f.Project, "gastown")
	}
}

func TestBrowseUpdate_MeKey(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	_, cmd := m.update(keyMsg("m"), Config{})
	if cmd == nil {
		t.Fatal("after 'm': expected a cmd, got nil")
	}
	msg := cmd()
	nav, ok := msg.(navigateMsg)
	if !ok {
		t.Fatalf("expected navigateMsg, got %T", msg)
	}
	if nav.view != viewMe {
		t.Errorf("expected viewMe, got %d", nav.view)
	}
}

func TestBrowseUpdate_SearchMode(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	m2, _ := m.update(keyMsg("/"), Config{})
	if !m2.searchMode {
		t.Error("after '/': searchMode should be true")
	}
	if !m2.search.Focused() {
		t.Error("after '/': search input should be focused")
	}
}

func TestBrowseView_StatusLabel(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	m.width = 80
	m.height = 24

	v := m.view()
	if !strings.Contains(v, "Status: open") {
		t.Errorf("initial view should show 'Status: open', got:\n%s", v)
	}

	m.statusIdx = 1
	v = m.view()
	if !strings.Contains(v, "Status: claimed") {
		t.Errorf("after statusIdx=1, view should show 'Status: claimed', got:\n%s", v)
	}

	m.statusIdx = 4 // "" → "all"
	v = m.view()
	if !strings.Contains(v, "Status: all") {
		t.Errorf("after statusIdx=4, view should show 'Status: all', got:\n%s", v)
	}
}

func TestBrowseView_SearchMode(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	m.width = 80
	m.height = 24
	m.searchMode = true
	m.search.Focus()

	v := m.view()
	// The search input placeholder or cursor should appear in the view.
	if !strings.Contains(v, "search") {
		t.Errorf("search mode view should contain search placeholder, got:\n%s", v)
	}
}

func TestBrowseView_TwoLineFilterBar(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	m.width = 80
	m.height = 24

	v := m.view()
	if !strings.Contains(v, "[s] Status:") {
		t.Errorf("view should contain first filter line, got:\n%s", v)
	}
	if !strings.Contains(v, "[p] Priority:") {
		t.Errorf("view should contain second filter line, got:\n%s", v)
	}
	if !strings.Contains(v, "Mine:") {
		t.Errorf("view should contain Mine filter, got:\n%s", v)
	}
	if !strings.Contains(v, "Sort:") {
		t.Errorf("view should contain Sort filter, got:\n%s", v)
	}
}

func TestBrowseView_StatusColumn(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	m.width = 80
	m.height = 24

	v := m.view()
	if !strings.Contains(v, "STATUS") {
		t.Errorf("view should contain STATUS column header, got:\n%s", v)
	}
	if strings.Contains(v, "EFFORT") {
		t.Errorf("view should NOT contain EFFORT column header, got:\n%s", v)
	}
}

func TestBrowseFilter_MyItems(t *testing.T) {
	m := newBrowseModel()
	m.myItems = true
	f := m.filter("test-rig")
	if f.MyItems != "test-rig" {
		t.Errorf("MyItems = %q, want %q", f.MyItems, "test-rig")
	}
}

func TestBrowseFilter_MyItemsDisabled(t *testing.T) {
	m := newBrowseModel()
	m.myItems = false
	f := m.filter("test-rig")
	if f.MyItems != "" {
		t.Errorf("MyItems should be empty when disabled, got %q", f.MyItems)
	}
}

func TestBrowseFilter_ProjectFilter_UsesStoredValue(t *testing.T) {
	m := newBrowseModel()
	// Directly set the stored filter value (not the textinput).
	m.projectFilter = "gastown"
	f := m.filter("test-rig")
	if f.Project != "gastown" {
		t.Errorf("filter Project = %q, want %q", f.Project, "gastown")
	}
}

func TestBrowseFilter_ProjectFilter_EmptyWhenUnset(t *testing.T) {
	m := newBrowseModel()
	f := m.filter("test-rig")
	if f.Project != "" {
		t.Errorf("filter Project should be empty, got %q", f.Project)
	}
}

func TestBrowseView_BranchIndicator(t *testing.T) {
	m := newBrowseModel()
	m.loading = false
	m.width = 80
	m.height = 24
	m.items = []commons.WantedSummary{
		{ID: "w-abc123", Title: "Has branch", Status: "claimed", Priority: 1, Project: "proj", Type: "bug"},
		{ID: "w-def456", Title: "No branch", Status: "open", Priority: 2, Project: "proj", Type: "bug"},
	}
	m.pendingIDs = map[string]int{"w-abc123": 1}

	v := m.view()
	// The item with a pending change should have * after its status.
	if !strings.Contains(v, "claimed*") {
		t.Errorf("view should contain 'claimed*' for pending item, got:\n%s", v)
	}
	// The item without pending changes should not have *.
	// "open" should appear without * (we check it doesn't have "open*").
	if strings.Contains(v, "open*") {
		t.Errorf("view should NOT contain 'open*' for non-pending item, got:\n%s", v)
	}
}

func TestBrowseSetData_StoresPendingIDs(t *testing.T) {
	m := newBrowseModel()
	pendingIDs := map[string]int{"w-abc123": 1}
	m.setData(browseDataMsg{
		items:      []commons.WantedSummary{{ID: "w-abc123", Status: "claimed"}},
		pendingIDs: pendingIDs,
	})
	if m.pendingIDs["w-abc123"] != 1 {
		t.Error("pendingIDs should contain w-abc123 with count 1")
	}
}
