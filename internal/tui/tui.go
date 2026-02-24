package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/julianknutsen/wasteland/internal/commons"
)

// Config holds the parameters needed to launch the TUI.
type Config struct {
	DBDir     string // local dolt database directory
	RigHandle string // current rig handle
	Upstream  string // upstream identifier for display
	Mode      string // "wild-west" or "pr"
	Signing   bool   // GPG-signed dolt commits
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
		detail: newDetailModel(cfg.DBDir, cfg.RigHandle, cfg.Mode),
		bar:    newStatusBar(fmt.Sprintf("%s@%s", cfg.RigHandle, cfg.Upstream)),
	}
}

// Init starts the initial data load.
func (m Model) Init() bubbletea.Cmd {
	return fetchBrowse(m.cfg, m.browse.filter())
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
			return m, fetchDetail(m.cfg, msg.wantedID)
		case viewBrowse:
			return m, fetchBrowse(m.cfg, m.browse.filter())
		}

	case browseDataMsg:
		m.browse.setData(msg)
		return m, nil

	case detailDataMsg:
		m.detail.setData(msg)
		return m, nil

	case actionRequestMsg:
		if m.cfg.Mode == "pr" {
			// PR mode: skip confirmation, execute immediately.
			m.detail.executing = true
			m.detail.executingLabel = commons.TransitionLabel(msg.transition)
			m.detail.result = ""
			m.detail.refreshViewport()
			return m, bubbletea.Batch(
				m.detail.spinner.Tick,
				executeMutation(m.cfg, m.detail.item.ID, msg.transition),
			)
		}
		// Wild-west: show confirmation prompt.
		m.detail.confirming = &confirmAction{
			transition: msg.transition,
			label:      msg.label,
		}
		m.detail.result = ""
		m.detail.refreshViewport()
		return m, nil

	case actionConfirmedMsg:
		m.detail.confirming = nil
		m.detail.executing = true
		m.detail.executingLabel = commons.TransitionLabel(msg.transition)
		m.detail.refreshViewport()
		return m, bubbletea.Batch(
			m.detail.spinner.Tick,
			executeMutation(m.cfg, m.detail.item.ID, msg.transition),
		)

	case actionResultMsg:
		m.detail.executing = false
		m.detail.executingLabel = ""
		if msg.err != nil {
			m.detail.result = styleError.Render("Error: " + msg.err.Error())
			m.detail.refreshViewport()
			return m, nil
		}
		if msg.detail != nil {
			// PR mode: apply the detail read from the branch so the
			// view reflects the updated state even though main hasn't changed.
			m.detail.setData(*msg.detail)
			m.detail.result = styleSuccess.Render("Pushed to " + msg.hint)
			m.detail.refreshViewport()
			return m, nil
		}
		// Wild-west: re-fetch detail to show updated status on main.
		m.detail.result = ""
		m.detail.refreshViewport()
		return m, fetchDetail(m.cfg, m.detail.item.ID)

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	// Delegate to active view.
	var cmd bubbletea.Cmd
	switch m.active {
	case viewBrowse:
		m.browse, cmd = m.browse.update(msg, m.cfg)
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
		hints = "esc: back  j/k: scroll  c/u/x/X/D: actions  q: quit"
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

func fetchBrowse(cfg Config, f commons.BrowseFilter) bubbletea.Cmd {
	return func() bubbletea.Msg {
		items, err := commons.BrowseWantedBranchAware(cfg.DBDir, cfg.Mode, cfg.RigHandle, f)
		if err != nil {
			return browseDataMsg{err: err}
		}
		return browseDataMsg{items: items}
	}
}

func executeMutation(cfg Config, wantedID string, t commons.Transition) bubbletea.Cmd {
	return func() bubbletea.Msg {
		// PR mode: checkout a per-item branch, mutate there, push branch, return to main.
		if cfg.Mode == "pr" {
			return executePRMutation(cfg, wantedID, t)
		}
		return executeWildWestMutation(cfg, wantedID, t)
	}
}

func executeWildWestMutation(cfg Config, wantedID string, t commons.Transition) actionResultMsg {
	if err := applyTransition(cfg, wantedID, t); err != nil {
		return actionResultMsg{err: err}
	}
	err := commons.PushWithSync(cfg.DBDir, io.Discard)
	return actionResultMsg{err: err}
}

func executePRMutation(cfg Config, wantedID string, t commons.Transition) actionResultMsg {
	branch := commons.BranchName(cfg.RigHandle, wantedID)

	if err := commons.CheckoutBranch(cfg.DBDir, branch); err != nil {
		return actionResultMsg{err: fmt.Errorf("checkout branch: %w", err)}
	}

	if err := applyTransition(cfg, wantedID, t); err != nil {
		_ = commons.CheckoutMain(cfg.DBDir)
		return actionResultMsg{err: err}
	}

	// Read updated detail while still on the branch.
	detail := fetchDetailSync(cfg.DBDir, wantedID)
	detail.branch = branch

	var pushLog bytes.Buffer
	if err := commons.PushBranch(cfg.DBDir, branch, &pushLog); err != nil {
		_ = commons.CheckoutMain(cfg.DBDir)
		// PushBranch writes the dolt error to stdout; surface it.
		if msg := strings.TrimSpace(pushLog.String()); msg != "" {
			return actionResultMsg{err: fmt.Errorf("%s", msg)}
		}
		return actionResultMsg{err: fmt.Errorf("push branch: %w", err)}
	}

	_ = commons.CheckoutMain(cfg.DBDir)
	return actionResultMsg{hint: branch, detail: &detail}
}

func applyTransition(cfg Config, wantedID string, t commons.Transition) error {
	switch t {
	case commons.TransitionClaim:
		return commons.ClaimWanted(cfg.DBDir, wantedID, cfg.RigHandle, cfg.Signing)
	case commons.TransitionUnclaim:
		return commons.UnclaimWanted(cfg.DBDir, wantedID, cfg.Signing)
	case commons.TransitionReject:
		return commons.RejectCompletion(cfg.DBDir, wantedID, "", "", cfg.Signing)
	case commons.TransitionClose:
		return commons.CloseWanted(cfg.DBDir, wantedID, cfg.Signing)
	case commons.TransitionDelete:
		return commons.DeleteWanted(cfg.DBDir, wantedID, cfg.Signing)
	default:
		return fmt.Errorf("unsupported transition")
	}
}

func fetchDetail(cfg Config, wantedID string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		// In PR mode, check if a mutation branch exists for this item.
		// If so, checkout the branch to read its state, then return to main.
		if cfg.Mode == "pr" {
			if branch := commons.FindBranchForItem(cfg.DBDir, cfg.RigHandle, wantedID); branch != "" {
				if err := commons.CheckoutBranch(cfg.DBDir, branch); err == nil {
					detail := fetchDetailSync(cfg.DBDir, wantedID)
					detail.branch = branch
					_ = commons.CheckoutMain(cfg.DBDir)
					return detail
				}
			}
		}
		return fetchDetailSync(cfg.DBDir, wantedID)
	}
}

// fetchDetailSync queries detail data from the current working copy.
func fetchDetailSync(dbDir, wantedID string) detailDataMsg {
	item, completion, stamp, err := commons.QueryFullDetail(dbDir, wantedID)
	if err != nil {
		return detailDataMsg{err: err}
	}
	return detailDataMsg{item: item, completion: completion, stamp: stamp}
}
