package style

import (
	"strings"
	"testing"
)

func TestSetColorMode_Never(t *testing.T) {
	SetColorMode("never")
	got := Success.Render("x")
	if strings.Contains(got, "\x1b") {
		t.Errorf("SetColorMode(never): Success.Render(\"x\") = %q, want no ANSI escapes", got)
	}
	if got != "x" {
		t.Errorf("SetColorMode(never): Success.Render(\"x\") = %q, want \"x\"", got)
	}
}

func TestSetColorMode_Always(t *testing.T) {
	SetColorMode("always")
	// Should not panic, styles should be re-initialized with colors.
	got := Success.Render("ok")
	if got == "" {
		t.Error("SetColorMode(always): Success.Render returned empty string")
	}
}

func TestSetColorMode_Auto(t *testing.T) {
	// auto is a no-op; just ensure it doesn't panic.
	SetColorMode("auto")
	got := Bold.Render("hi")
	if got == "" {
		t.Error("SetColorMode(auto): Bold.Render returned empty string")
	}
}
