package tui

import (
	"fmt"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/julianknutsen/wasteland/internal/commons"
)

// Config holds the parameters needed to launch the TUI.
type Config struct {
	DBDir     string // local dolt database directory
	RigHandle string // current rig handle
	Upstream  string // upstream identifier for display
}

// Model is the root TUI model that routes between views.
type Model struct {
	cfg      Config
	active   activeView
	browse   browseModel
	detail   detailModel
	bar      statusBar
	width    int
	height   int
	err      error
	quitting bool
}

// New creates a new root TUI model.
func New(cfg Config) Model {
	return Model{
		cfg:    cfg,
		active: viewBrowse,
		browse: newBrowseModel(),
		detail: newDetailModel(),
		bar:    newStatusBar(fmt.Sprintf("%s@%s", cfg.RigHandle, cfg.Upstream)),
	}
}

// Init starts the initial data load.
func (m Model) Init() bubbletea.Cmd {
	return fetchBrowse(m.cfg.DBDir, m.browse.filter())
}

// Update processes messages.
func (m Model) Update(msg bubbletea.Msg) (bubbletea.Model, bubbletea.Cmd) {
	switch msg := msg.(type) {
	case bubbletea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, bubbletea.Quit
		}

	case bubbletea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.bar.width = msg.Width
		m.browse.setSize(msg.Width, msg.Height-1) // -1 for statusbar
		m.detail.setSize(msg.Width, msg.Height-1)

	case navigateMsg:
		m.active = msg.view
		switch msg.view {
		case viewDetail:
			return m, fetchDetail(m.cfg.DBDir, msg.wantedID)
		case viewBrowse:
			return m, fetchBrowse(m.cfg.DBDir, m.browse.filter())
		}

	case browseDataMsg:
		m.browse.setData(msg)
		return m, nil

	case detailDataMsg:
		m.detail.setData(msg)
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	// Delegate to active view.
	var cmd bubbletea.Cmd
	switch m.active {
	case viewBrowse:
		m.browse, cmd = m.browse.update(msg, m.cfg.DBDir)
	case viewDetail:
		m.detail, cmd = m.detail.update(msg)
	}
	return m, cmd
}

// View renders the current view.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var content string
	var hints string

	switch m.active {
	case viewBrowse:
		content = m.browse.view()
		hints = "j/k: navigate  enter: open  /: search  s: cycle status  t: cycle type  q: quit"
	case viewDetail:
		content = m.detail.view()
		hints = "esc: back  j/k: scroll  q: quit"
	}

	// Pad content to fill available height.
	contentHeight := m.height - 1 // 1 for statusbar
	content = lipgloss.NewStyle().
		Width(m.width).
		Height(contentHeight).
		Render(content)

	bar := m.bar.render(hints)

	return content + "\n" + bar
}

// --- async commands ---

func fetchBrowse(dbDir string, f commons.BrowseFilter) bubbletea.Cmd {
	return func() bubbletea.Msg {
		items, err := commons.BrowseWanted(dbDir, f)
		return browseDataMsg{items: items, err: err}
	}
}

func fetchDetail(dbDir, wantedID string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		store := commons.NewWLCommons(dbDir)
		item, err := store.QueryWantedDetail(wantedID)
		if err != nil {
			return detailDataMsg{err: err}
		}
		msg := detailDataMsg{item: item}

		if item.Status == "in_review" || item.Status == "completed" {
			if completion, err := store.QueryCompletion(wantedID); err == nil {
				msg.completion = completion
				if completion.StampID != "" {
					if stamp, err := store.QueryStamp(completion.StampID); err == nil {
						msg.stamp = stamp
					}
				}
			}
		}

		return msg
	}
}
