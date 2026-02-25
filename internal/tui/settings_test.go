package tui

import (
	"fmt"
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

func settingsClient(saveErr error) *sdk.Client {
	return sdk.New(sdk.ClientConfig{
		SaveConfig: func(_ string, _ bool) error { return saveErr },
	})
}

func TestSettings_Toggle_Mode(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.cursor = 0 // mode

	cfg := Config{Client: settingsClient(nil)}
	m2, cmd := m.toggle(cfg)

	if m2.mode != "pr" {
		t.Errorf("mode = %q, want %q", m2.mode, "pr")
	}
	if cmd == nil {
		t.Fatal("expected cmd from toggle")
	}

	msg := cmd()
	saved, ok := msg.(settingsSavedMsg)
	if !ok {
		t.Fatalf("expected settingsSavedMsg, got %T", msg)
	}
	if saved.mode != "pr" {
		t.Errorf("saved mode = %q, want %q", saved.mode, "pr")
	}
	if saved.err != nil {
		t.Errorf("unexpected error: %v", saved.err)
	}
}

func TestSettings_Toggle_ModePRToWildWest(t *testing.T) {
	m := newSettingsModel("pr", false)
	m.cursor = 0

	cfg := Config{Client: settingsClient(nil)}
	m2, _ := m.toggle(cfg)

	if m2.mode != "wild-west" {
		t.Errorf("mode = %q, want %q", m2.mode, "wild-west")
	}
}

func TestSettings_Toggle_Signing(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.cursor = 1 // signing

	cfg := Config{Client: settingsClient(nil)}
	m2, cmd := m.toggle(cfg)

	if !m2.signing {
		t.Error("signing should be true after toggle")
	}
	if cmd == nil {
		t.Fatal("expected cmd from toggle")
	}

	msg := cmd()
	saved := msg.(settingsSavedMsg)
	if !saved.signing {
		t.Error("saved signing should be true")
	}
}

func TestSettings_Toggle_SaveError(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.cursor = 0

	cfg := Config{Client: settingsClient(fmt.Errorf("disk full"))}
	_, cmd := m.toggle(cfg)

	msg := cmd()
	saved := msg.(settingsSavedMsg)
	if saved.err == nil {
		t.Fatal("expected error from save")
	}
	if !strings.Contains(saved.err.Error(), "disk full") {
		t.Errorf("error = %q, want to contain %q", saved.err.Error(), "disk full")
	}
}

func TestSettings_Toggle_NilClient(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.cursor = 0

	cfg := Config{} // Client is nil
	m2, cmd := m.toggle(cfg)

	if m2.mode != "pr" {
		t.Errorf("mode = %q, want %q", m2.mode, "pr")
	}
	if cmd == nil {
		t.Fatal("expected cmd from toggle")
	}

	msg := cmd()
	saved := msg.(settingsSavedMsg)
	if saved.err != nil {
		t.Errorf("unexpected error: %v", saved.err)
	}
}

func TestSettings_Cursor_Navigation(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	cfg := Config{}

	// Start at 0, pressing down moves to 1.
	m2, _ := m.update(keyMsg("j"), cfg)
	if m2.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m2.cursor)
	}

	// Can't go below 1.
	m3, _ := m2.update(keyMsg("j"), cfg)
	if m3.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (clamped)", m3.cursor)
	}

	// Press up goes back to 0.
	m4, _ := m3.update(keyMsg("k"), cfg)
	if m4.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m4.cursor)
	}

	// Can't go above 0.
	m5, _ := m4.update(keyMsg("k"), cfg)
	if m5.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m5.cursor)
	}
}

func TestSettings_Enter_Toggles(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.cursor = 0
	cfg := Config{Client: settingsClient(nil)}

	m2, cmd := m.update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter}, cfg)
	if m2.mode != "pr" {
		t.Errorf("mode = %q, want %q", m2.mode, "pr")
	}
	if cmd == nil {
		t.Fatal("expected cmd from enter")
	}
}

func TestSettings_LeftRight_Toggles(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.cursor = 0
	cfg := Config{Client: settingsClient(nil)}

	m2, cmd := m.update(bubbletea.KeyMsg{Type: bubbletea.KeyRight}, cfg)
	if m2.mode != "pr" {
		t.Errorf("mode = %q, want %q after right", m2.mode, "pr")
	}
	if cmd == nil {
		t.Fatal("expected cmd from right key")
	}
}

func TestSettings_Esc_ReturnsNavigateMsg(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	cfg := Config{}

	_, cmd := m.update(bubbletea.KeyMsg{Type: bubbletea.KeyEsc}, cfg)
	if cmd == nil {
		t.Fatal("expected cmd from esc")
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

func TestSettings_View_ShowsReadOnlyFields(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.width = 80
	m.height = 24

	cfg := Config{
		RigHandle:    "myrig",
		Upstream:     "steve/wl-commons",
		ProviderType: "dolthub",
		ForkOrg:      "myrig-dev",
		ForkDB:       "wl-commons",
		LocalDir:     "/home/user/.local/share/wasteland/test",
		JoinedAt:     "2025-12-01",
		Mode:         "wild-west",
		Signing:      false,
	}

	v := m.view(cfg)
	for _, want := range []string{
		"Settings",
		"myrig",
		"steve/wl-commons",
		"dolthub",
		"myrig-dev/wl-commons",
		"/home/user/.local/share/wasteland/test",
		"2025-12-01",
		"[wild-west]",
		"[false]",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view should contain %q, got:\n%s", want, v)
		}
	}
}

func TestSettings_View_ShowsSavedResult(t *testing.T) {
	m := newSettingsModel("pr", true)
	m.result = styleSuccess.Render("Saved")
	m.width = 80
	m.height = 24

	cfg := Config{Mode: "pr", Signing: true}
	v := m.view(cfg)
	if !strings.Contains(v, "Saved") {
		t.Errorf("view should contain 'Saved', got:\n%s", v)
	}
}

func TestSettings_Sync(t *testing.T) {
	m := newSettingsModel("wild-west", false)
	m.result = "stale"
	m.sync("pr", true)

	if m.mode != "pr" {
		t.Errorf("mode = %q, want %q", m.mode, "pr")
	}
	if !m.signing {
		t.Error("signing should be true after sync")
	}
	if m.result != "" {
		t.Errorf("result should be cleared, got %q", m.result)
	}
}
