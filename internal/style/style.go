// Package style provides consistent terminal styling using Lipgloss.
// Uses the Ayu theme colors inlined directly (no separate ui package).
package style

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Ayu theme color palette (inlined from gastown's internal/ui/styles.go)
var (
	colorPass = lipgloss.AdaptiveColor{
		Light: "#86b300",
		Dark:  "#c2d94c",
	}
	colorWarn = lipgloss.AdaptiveColor{
		Light: "#f2ae49",
		Dark:  "#ffb454",
	}
	colorFail = lipgloss.AdaptiveColor{
		Light: "#f07171",
		Dark:  "#f07178",
	}
	colorMuted = lipgloss.AdaptiveColor{
		Light: "#828c99",
		Dark:  "#6c7680",
	}
	colorAccent = lipgloss.AdaptiveColor{
		Light: "#399ee6",
		Dark:  "#59c2ff",
	}
)

// Semantic icons
const (
	IconPass = "✓"
	IconWarn = "⚠"
	IconFail = "✖"
)

var (
	// Success style for positive outcomes (green)
	Success = lipgloss.NewStyle().
		Foreground(colorPass).
		Bold(true)

	// Warning style for cautionary messages (yellow)
	Warning = lipgloss.NewStyle().
		Foreground(colorWarn).
		Bold(true)

	// Error style for failures (red)
	Error = lipgloss.NewStyle().
		Foreground(colorFail).
		Bold(true)

	// Info style for informational messages (blue)
	Info = lipgloss.NewStyle().
		Foreground(colorAccent)

	// Dim style for secondary information (gray)
	Dim = lipgloss.NewStyle().
		Foreground(colorMuted)

	// Bold style for emphasis
	Bold = lipgloss.NewStyle().
		Bold(true)
)

// SetColorMode overrides style rendering based on --color flag or NO_COLOR env.
func SetColorMode(mode string) {
	switch mode {
	case "never":
		_ = os.Setenv("NO_COLOR", "1")
		Success = lipgloss.NewStyle()
		Warning = lipgloss.NewStyle()
		Error = lipgloss.NewStyle()
		Info = lipgloss.NewStyle()
		Dim = lipgloss.NewStyle()
		Bold = lipgloss.NewStyle()
	case "always":
		_ = os.Unsetenv("NO_COLOR")
		_ = os.Setenv("CLICOLOR_FORCE", "1")
		Success = lipgloss.NewStyle().Foreground(colorPass).Bold(true)
		Warning = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
		Error = lipgloss.NewStyle().Foreground(colorFail).Bold(true)
		Info = lipgloss.NewStyle().Foreground(colorAccent)
		Dim = lipgloss.NewStyle().Foreground(colorMuted)
		Bold = lipgloss.NewStyle().Bold(true)
	}
}
