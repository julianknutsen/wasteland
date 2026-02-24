package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// statusBar renders the bottom bar showing handle, context, and key hints.
type statusBar struct {
	handle string
	width  int
}

func newStatusBar(handle string) statusBar {
	return statusBar{handle: handle}
}

func (s statusBar) render(hints string) string {
	left := styleDim.Render(s.handle)
	right := styleDim.Render(hints)

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return styleBar.Width(s.width).Render(
		fmt.Sprintf("%s%*s%s", left, gap, "", right),
	)
}
