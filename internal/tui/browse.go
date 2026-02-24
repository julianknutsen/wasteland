// Package tui provides an interactive terminal UI for the Wasteland wanted board.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/julianknutsen/wasteland/internal/commons"
)

type browseModel struct {
	items      []commons.WantedSummary
	cursor     int
	statusIdx  int // index into statusCycle
	typeIdx    int // index into typeCycle
	searchMode bool
	search     textinput.Model
	width      int
	height     int
	loading    bool
	err        error
}

func newBrowseModel() browseModel {
	ti := textinput.New()
	ti.Placeholder = "search title..."
	ti.CharLimit = 64
	return browseModel{
		statusIdx: 0, // default to "open"
		search:    ti,
		loading:   true,
	}
}

func (m browseModel) filter() commons.BrowseFilter {
	return commons.BrowseFilter{
		Status:   commons.ValidStatuses()[m.statusIdx],
		Type:     commons.ValidTypes()[m.typeIdx],
		Priority: -1,
		Limit:    100,
		Search:   m.search.Value(),
	}
}

func (m *browseModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *browseModel) setData(msg browseDataMsg) {
	m.loading = false
	m.err = msg.err
	m.items = msg.items
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

func (m browseModel) update(msg bubbletea.Msg, cfg Config) (browseModel, bubbletea.Cmd) {
	if m.searchMode {
		return m.updateSearch(msg, cfg)
	}

	if msg, ok := msg.(bubbletea.KeyMsg); ok {
		switch {
		case key.Matches(msg, keys.Quit):
			return m, bubbletea.Quit

		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case key.Matches(msg, keys.Enter):
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				return m, func() bubbletea.Msg {
					return navigateMsg{view: viewDetail, wantedID: item.ID}
				}
			}

		case key.Matches(msg, keys.Search):
			m.searchMode = true
			m.search.Focus()
			return m, textinput.Blink

		case key.Matches(msg, keys.Status):
			m.statusIdx = (m.statusIdx + 1) % len(commons.ValidStatuses())
			m.cursor = 0
			m.loading = true
			return m, fetchBrowse(cfg, m.filter())

		case key.Matches(msg, keys.Type):
			m.typeIdx = (m.typeIdx + 1) % len(commons.ValidTypes())
			m.cursor = 0
			m.loading = true
			return m, fetchBrowse(cfg, m.filter())
		}
	}

	return m, nil
}

func (m browseModel) updateSearch(msg bubbletea.Msg, cfg Config) (browseModel, bubbletea.Cmd) {
	if msg, ok := msg.(bubbletea.KeyMsg); ok {
		switch msg.String() {
		case "enter", "esc":
			m.searchMode = false
			m.search.Blur()
			if msg.String() == "enter" {
				m.cursor = 0
				m.loading = true
				return m, fetchBrowse(cfg, m.filter())
			}
			return m, nil
		}
	}

	var cmd bubbletea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m browseModel) view() string {
	var b strings.Builder

	// Title line.
	b.WriteString(styleTitle.Render("Wasteland Board"))
	b.WriteByte('\n')

	// Filter bar — always visible so user can see active filters.
	statusLabel := commons.StatusLabel(commons.ValidStatuses()[m.statusIdx])
	typeLabel := commons.TypeLabel(commons.ValidTypes()[m.typeIdx])
	filterLine := fmt.Sprintf("  [s] Status: %-12s  [t] Type: %-10s", statusLabel, typeLabel)
	if m.search.Value() != "" {
		filterLine += fmt.Sprintf("  Search: %q", m.search.Value())
	}
	b.WriteString(styleFilterBar.Render(filterLine))
	b.WriteByte('\n')

	// Search bar — shown on its own line when active.
	if m.searchMode {
		b.WriteString("  Search: ")
		b.WriteString(m.search.View())
		b.WriteByte('\n')
	}

	// Column headers.
	colHeader := fmt.Sprintf("  %-12s %-36s %-10s %-8s %-4s %-8s",
		"ID", "TITLE", "PROJECT", "TYPE", "PRI", "EFFORT")
	b.WriteString(styleDim.Render(colHeader))
	b.WriteByte('\n')

	// Separator.
	sep := strings.Repeat("─", min(m.width, lipgloss.Width(colHeader)+2))
	b.WriteString(styleDim.Render(sep))
	b.WriteByte('\n')

	if m.loading {
		b.WriteString(styleDim.Render("  Loading..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v", m.err))
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(styleDim.Render(fmt.Sprintf(
			"  No %s items found (type: %s).", statusLabel, typeLabel)))
		return b.String()
	}

	// Item count.
	b.WriteString(styleDim.Render(fmt.Sprintf("  %d items", len(m.items))))
	b.WriteByte('\n')

	// Compute visible window.
	headerLines := 6 // title + filter + colheader + sep + count + slack
	if m.searchMode {
		headerLines++
	}
	listHeight := m.height - headerLines
	if listHeight < 1 {
		listHeight = 10
	}
	startIdx := 0
	if m.cursor >= listHeight {
		startIdx = m.cursor - listHeight + 1
	}
	endIdx := startIdx + listHeight
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	for i := startIdx; i < endIdx; i++ {
		item := m.items[i]
		title := item.Title
		if len(title) > 36 {
			title = title[:33] + "..."
		}
		pri := colorizePriority(item.Priority)
		line := fmt.Sprintf("  %-12s %-36s %-10s %-8s %-4s %-8s",
			item.ID, title, item.Project, item.Type, pri, item.EffortLevel)

		if i == m.cursor {
			line = styleSelected.Width(m.width).Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return b.String()
}
