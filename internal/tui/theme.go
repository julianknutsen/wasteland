package tui

import "github.com/charmbracelet/lipgloss"

// Ayu theme colors for TUI contexts.
var (
	colorPass = lipgloss.AdaptiveColor{Light: "#86b300", Dark: "#c2d94c"}
	colorWarn = lipgloss.AdaptiveColor{Light: "#f2ae49", Dark: "#ffb454"}
	colorFail = lipgloss.AdaptiveColor{Light: "#f07171", Dark: "#f07178"}
	colorDim  = lipgloss.AdaptiveColor{Light: "#828c99", Dark: "#6c7680"}
	colorText = lipgloss.AdaptiveColor{Light: "#5c6166", Dark: "#bfbdb6"}
	colorSel  = lipgloss.AdaptiveColor{Light: "#e8e8e8", Dark: "#1a1f29"}
)

var (
	styleTitle = lipgloss.NewStyle().Bold(true)

	styleSelected = lipgloss.NewStyle().
			Background(colorSel).
			Foreground(colorText)

	styleDim = lipgloss.NewStyle().Foreground(colorDim)

	styleStatusOpen     = lipgloss.NewStyle().Foreground(colorText)
	styleStatusClaimed  = lipgloss.NewStyle().Foreground(colorWarn)
	styleStatusReview   = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	styleStatusComplete = lipgloss.NewStyle().Foreground(colorPass)

	styleBar = lipgloss.NewStyle().
			Background(colorSel).
			Foreground(colorDim).
			Padding(0, 1)

	styleFilterBar = lipgloss.NewStyle().Foreground(colorText)

	styleP0 = lipgloss.NewStyle().Foreground(colorFail).Bold(true)
	styleP1 = lipgloss.NewStyle().Foreground(colorWarn)
)

func colorizeStatus(status string) string {
	switch status {
	case "open":
		return styleStatusOpen.Render(status)
	case "claimed":
		return styleStatusClaimed.Render(status)
	case "in_review":
		return styleStatusReview.Render(status)
	case "completed":
		return styleStatusComplete.Render(status)
	default:
		return styleDim.Render(status)
	}
}

func colorizePriority(pri int) string {
	switch pri {
	case 0:
		return styleP0.Render("P0")
	case 1:
		return styleP1.Render("P1")
	default:
		return styleDim.Render("P" + string(rune('0'+pri)))
	}
}
