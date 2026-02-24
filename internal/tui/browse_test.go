package tui

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
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

	m2, cmd := m.update(keyMsg("s"), "/tmp/fake")
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

	m2, cmd := m.update(keyMsg("t"), "/tmp/fake")
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

func TestBrowseUpdate_SearchMode(t *testing.T) {
	m := newBrowseModel()
	m.loading = false

	m2, _ := m.update(keyMsg("/"), "/tmp/fake")
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
