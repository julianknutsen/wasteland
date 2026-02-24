package tui

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
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
