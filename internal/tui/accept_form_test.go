package tui

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
)

func TestAcceptForm_NewAcceptForm(t *testing.T) {
	f := newAcceptForm()
	if !f.active {
		t.Error("new accept form should be active")
	}
	if f.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", f.cursor)
	}
	if f.severityIdx != 0 {
		t.Errorf("severityIdx should start at 0, got %d", f.severityIdx)
	}
}

func TestAcceptForm_EmptyQuality_ShowsError(t *testing.T) {
	f := newAcceptForm()

	result, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if result == nil {
		t.Fatal("form should not be canceled on validation error")
	}
	if result.err == "" {
		t.Error("should show validation error for empty quality")
	}
	if !strings.Contains(result.err, "quality must be 1-5") {
		t.Errorf("error should mention quality, got %q", result.err)
	}
	if cmd != nil {
		t.Error("should not return cmd on validation failure")
	}
}

func TestAcceptForm_InvalidQuality_ShowsError(t *testing.T) {
	f := newAcceptForm()
	// Type invalid quality.
	f.update(keyMsg("9"))

	result, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if result == nil {
		t.Fatal("form should not be canceled")
	}
	if !strings.Contains(result.err, "quality must be 1-5") {
		t.Errorf("error should mention quality, got %q", result.err)
	}
	if cmd != nil {
		t.Error("should not return cmd on validation failure")
	}
}

func TestAcceptForm_ValidSubmit_ReturnsAcceptSubmitMsg(t *testing.T) {
	f := newAcceptForm()

	// Type quality = 4.
	f.update(keyMsg("4"))

	// Tab to reliability.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	// Type reliability = 3.
	f.update(keyMsg("3"))

	// Tab to severity (cursor=2), then right to "branch".
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyRight})

	// Tab to skills.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	for _, ch := range "go, dolt" {
		f.update(keyMsg(string(ch)))
	}

	// Tab to message.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	for _, ch := range "solid work" {
		f.update(keyMsg(string(ch)))
	}

	// Submit.
	_, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return acceptSubmitMsg cmd")
	}

	msg := cmd()
	submit, ok := msg.(acceptSubmitMsg)
	if !ok {
		t.Fatalf("expected acceptSubmitMsg, got %T", msg)
	}
	if submit.quality != 4 {
		t.Errorf("quality = %d, want 4", submit.quality)
	}
	if submit.reliability != 3 {
		t.Errorf("reliability = %d, want 3", submit.reliability)
	}
	if submit.severity != "branch" {
		t.Errorf("severity = %q, want %q", submit.severity, "branch")
	}
	if len(submit.skills) != 2 || submit.skills[0] != "go" || submit.skills[1] != "dolt" {
		t.Errorf("skills = %v, want [go dolt]", submit.skills)
	}
	if submit.message != "solid work" {
		t.Errorf("message = %q, want %q", submit.message, "solid work")
	}
}

func TestAcceptForm_ReliabilityDefaultsToQuality(t *testing.T) {
	f := newAcceptForm()

	// Type quality = 5, leave reliability empty.
	f.update(keyMsg("5"))

	_, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return cmd")
	}

	msg := cmd()
	submit := msg.(acceptSubmitMsg)
	if submit.reliability != 5 {
		t.Errorf("reliability should default to quality (5), got %d", submit.reliability)
	}
}

func TestAcceptForm_Escape_CancelsForm(t *testing.T) {
	f := newAcceptForm()

	result, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	if result != nil {
		t.Error("esc should cancel form")
	}
	if cmd != nil {
		t.Error("esc should not return a cmd")
	}
}

func TestAcceptForm_TabNavigation(t *testing.T) {
	f := newAcceptForm()
	if f.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", f.cursor)
	}

	// Tab through fields.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if f.cursor != 1 {
		t.Errorf("after tab: cursor = %d, want 1", f.cursor)
	}

	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if f.cursor != 2 {
		t.Errorf("after tab: cursor = %d, want 2", f.cursor)
	}

	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if f.cursor != 3 {
		t.Errorf("after tab: cursor = %d, want 3", f.cursor)
	}

	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if f.cursor != 4 {
		t.Errorf("after tab: cursor = %d, want 4", f.cursor)
	}

	// Wrap around.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if f.cursor != 0 {
		t.Errorf("after wrap: cursor = %d, want 0", f.cursor)
	}
}

func TestAcceptForm_ShiftTabNavigation(t *testing.T) {
	f := newAcceptForm()

	// Shift-tab from 0 should wrap to 4.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyShiftTab})
	if f.cursor != 4 {
		t.Errorf("after shift-tab from 0: cursor = %d, want 4", f.cursor)
	}

	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyShiftTab})
	if f.cursor != 3 {
		t.Errorf("after shift-tab: cursor = %d, want 3", f.cursor)
	}
}

func TestAcceptForm_SeverityCycle(t *testing.T) {
	f := newAcceptForm()
	// Navigate to severity field.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if f.cursor != 2 {
		t.Fatalf("cursor should be 2, got %d", f.cursor)
	}

	if f.severityIdx != 0 {
		t.Fatalf("initial severity = %d, want 0 (leaf)", f.severityIdx)
	}

	// Right → branch.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyRight})
	if f.severityIdx != 1 {
		t.Errorf("after right: severity = %d, want 1 (branch)", f.severityIdx)
	}

	// Right → root.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyRight})
	if f.severityIdx != 2 {
		t.Errorf("after right: severity = %d, want 2 (root)", f.severityIdx)
	}

	// Right → wrap to leaf.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyRight})
	if f.severityIdx != 0 {
		t.Errorf("after right wrap: severity = %d, want 0 (leaf)", f.severityIdx)
	}

	// Left → wrap to root.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyLeft})
	if f.severityIdx != 2 {
		t.Errorf("after left wrap: severity = %d, want 2 (root)", f.severityIdx)
	}
}

func TestAcceptForm_View_ContainsFields(t *testing.T) {
	f := newAcceptForm()
	v := f.view()

	if !strings.Contains(v, "Accept:") {
		t.Errorf("view should contain 'Accept:', got:\n%s", v)
	}
	if !strings.Contains(v, "Quality:") {
		t.Errorf("view should contain 'Quality:', got:\n%s", v)
	}
	if !strings.Contains(v, "Reliability:") {
		t.Errorf("view should contain 'Reliability:', got:\n%s", v)
	}
	if !strings.Contains(v, "Severity:") {
		t.Errorf("view should contain 'Severity:', got:\n%s", v)
	}
	if !strings.Contains(v, "Skills:") {
		t.Errorf("view should contain 'Skills:', got:\n%s", v)
	}
	if !strings.Contains(v, "Message:") {
		t.Errorf("view should contain 'Message:', got:\n%s", v)
	}
	if !strings.Contains(v, "tab: fields") {
		t.Errorf("view should contain 'tab: fields', got:\n%s", v)
	}
	// First field (quality) should have cursor.
	if !strings.Contains(v, "> Quality:") {
		t.Errorf("view should show cursor on quality, got:\n%s", v)
	}
}

func TestAcceptForm_InvalidReliability_ShowsError(t *testing.T) {
	f := newAcceptForm()

	// Set valid quality.
	f.update(keyMsg("3"))

	// Tab to reliability and set invalid value.
	f.update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	f.update(keyMsg("8"))

	result, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if result == nil {
		t.Fatal("form should not be canceled on validation error")
	}
	if !strings.Contains(result.err, "reliability must be 1-5") {
		t.Errorf("error should mention reliability, got %q", result.err)
	}
	if cmd != nil {
		t.Error("should not return cmd on validation failure")
	}
}
