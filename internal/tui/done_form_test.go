package tui

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
)

func TestDoneForm_NewDoneForm(t *testing.T) {
	f := newDoneForm()
	if !f.active {
		t.Error("new done form should be active")
	}
	if f.evidence.Value() != "" {
		t.Errorf("evidence should be empty, got %q", f.evidence.Value())
	}
}

func TestDoneForm_EmptySubmit_ShowsError(t *testing.T) {
	f := newDoneForm()

	result, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if result == nil {
		t.Fatal("form should not be canceled on empty submit")
	}
	if result.err == "" {
		t.Error("should show validation error for empty evidence")
	}
	if !strings.Contains(result.err, "required") {
		t.Errorf("error should mention required, got %q", result.err)
	}
	if cmd != nil {
		t.Error("should not return cmd on validation failure")
	}
}

func TestDoneForm_ValidSubmit_ReturnsDoneSubmitMsg(t *testing.T) {
	f := newDoneForm()

	// Type evidence URL.
	for _, ch := range "https://example.com/pr/1" {
		f.update(keyMsg(string(ch)))
	}

	_, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return doneSubmitMsg cmd")
	}

	msg := cmd()
	submit, ok := msg.(doneSubmitMsg)
	if !ok {
		t.Fatalf("expected doneSubmitMsg, got %T", msg)
	}
	if submit.evidence != "https://example.com/pr/1" {
		t.Errorf("evidence = %q, want %q", submit.evidence, "https://example.com/pr/1")
	}
}

func TestDoneForm_Escape_CancelsForm(t *testing.T) {
	f := newDoneForm()

	result, cmd := f.update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc})
	if result != nil {
		t.Error("esc should cancel form (return nil)")
	}
	if cmd != nil {
		t.Error("esc should not return a cmd")
	}
}

func TestDoneForm_View_ContainsElements(t *testing.T) {
	f := newDoneForm()
	v := f.view()

	if !strings.Contains(v, "Done:") {
		t.Errorf("view should contain 'Done:', got:\n%s", v)
	}
	if !strings.Contains(v, "Evidence:") {
		t.Errorf("view should contain 'Evidence:', got:\n%s", v)
	}
	if !strings.Contains(v, "enter: submit") {
		t.Errorf("view should contain 'enter: submit', got:\n%s", v)
	}
	if !strings.Contains(v, "esc: cancel") {
		t.Errorf("view should contain 'esc: cancel', got:\n%s", v)
	}
}

func TestDoneForm_View_ShowsError(t *testing.T) {
	f := newDoneForm()
	f.err = "evidence URL is required"
	v := f.view()

	if !strings.Contains(v, "evidence URL is required") {
		t.Errorf("view should contain error message, got:\n%s", v)
	}
}
