package tui

import (
	"fmt"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

// Config holds the parameters needed to launch the TUI.
type Config struct {
	Client    *sdk.Client // SDK client for all operations
	RigHandle string      // current rig handle (display)
	Upstream  string      // upstream identifier for display
	Mode      string      // "wild-west" or "pr" (display and behavior)
	Signing   bool        // GPG-signed dolt commits (display)

	// Settings view: read-only context
	ProviderType string
	ForkOrg      string
	ForkDB       string
	LocalDir     string
	JoinedAt     string
}

// Model is the root TUI model that routes between views.
type Model struct {
	cfg      Config
	active   activeView
	browse   browseModel
	detail   detailModel
	me       meModel
	settings settingsModel
	bar      statusBar
	width    int
	height   int
	err      error
	quitting bool
}

// New creates a new root TUI model.
func New(cfg Config) Model {
	return Model{
		cfg:      cfg,
		active:   viewBrowse,
		browse:   newBrowseModel(),
		detail:   newDetailModel(cfg.RigHandle, cfg.Mode),
		me:       newMeModel(),
		settings: newSettingsModel(cfg.Mode, cfg.Signing),
		bar:      newStatusBar(fmt.Sprintf("%s@%s", cfg.RigHandle, cfg.Upstream)),
	}
}

// Init starts the initial data load.
func (m Model) Init() bubbletea.Cmd {
	return fetchBrowse(m.cfg, m.browse.filter(m.cfg.RigHandle))
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
		m.me.setSize(msg.Width, msg.Height-1)
		m.settings.setSize(msg.Width, msg.Height-1)

	case navigateMsg:
		m.active = msg.view
		switch msg.view {
		case viewDetail:
			return m, fetchDetail(m.cfg, msg.wantedID)
		case viewBrowse:
			return m, fetchBrowse(m.cfg, m.browse.filter(m.cfg.RigHandle))
		case viewMe:
			m.me.loading = true
			return m, fetchMe(m.cfg)
		case viewSettings:
			m.settings.sync(m.cfg.Mode, m.cfg.Signing)
			return m, nil
		}

	case browseDataMsg:
		m.browse.setData(msg)
		return m, nil

	case detailDataMsg:
		m.detail.setData(msg)
		return m, nil

	case meDataMsg:
		m.me.setData(msg)
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
		r := msg.result
		if r != nil && r.Detail != nil {
			m.detail.setData(sdkDetailToMsg(r.Detail))
		}
		if r != nil && r.Hint != "" {
			m.detail.result = styleSuccess.Render(r.Hint)
		} else if r != nil && r.Branch != "" {
			m.detail.result = styleSuccess.Render("Pushed to " + r.Branch)
		}
		m.detail.refreshViewport()
		return m, nil

	case deltaRequestMsg:
		m.detail.deltaConfirm = &deltaConfirmAction{
			action: msg.action,
			label:  msg.label,
		}
		m.detail.result = ""
		m.detail.refreshViewport()
		return m, nil

	case deltaConfirmedMsg:
		m.detail.deltaConfirm = nil
		m.detail.executing = true
		switch msg.action {
		case deltaApply:
			m.detail.executingLabel = "Applying..."
		case deltaDiscard:
			m.detail.executingLabel = "Discarding..."
		}
		m.detail.refreshViewport()
		return m, bubbletea.Batch(
			m.detail.spinner.Tick,
			executeDelta(m.cfg, m.detail.branch, msg.action),
		)

	case deltaResultMsg:
		m.detail.executing = false
		m.detail.executingLabel = ""
		if msg.err != nil {
			m.detail.result = styleError.Render("Error: " + msg.err.Error())
			m.detail.refreshViewport()
			return m, nil
		}
		if msg.hint == "discarded" {
			// Discard resolved — navigate back to browse.
			m.active = viewBrowse
			return m, fetchBrowse(m.cfg, m.browse.filter(m.cfg.RigHandle))
		}
		// Apply resolved — re-fetch detail from main (branch is gone).
		return m, fetchDetail(m.cfg, m.detail.item.ID)

	case doneSubmitMsg:
		m.detail.doneForm = nil
		m.detail.executing = true
		m.detail.executingLabel = "Submitting..."
		m.detail.refreshViewport()
		return m, bubbletea.Batch(
			m.detail.spinner.Tick,
			executeDoneMutation(m.cfg, m.detail.item.ID, msg.evidence),
		)

	case acceptSubmitMsg:
		m.detail.acceptForm = nil
		m.detail.executing = true
		m.detail.executingLabel = "Accepting..."
		m.detail.refreshViewport()
		return m, bubbletea.Batch(
			m.detail.spinner.Tick,
			executeAcceptMutation(m.cfg, m.detail.item.ID, msg),
		)

	case submitDiffMsg:
		if m.detail.submit != nil {
			m.detail.submit.setDiff(msg)
		}
		return m, nil

	case submitOpenedMsg:
		// Submit view opened — start loading diff in background.
		return m, fetchDiff(m.cfg, msg.branch)

	case submitConfirmMsg:
		if m.detail.submit == nil {
			return m, nil
		}
		m.detail.executing = true
		m.detail.executingLabel = "Creating PR..."
		m.detail.submit = nil
		m.detail.refreshViewport()
		return m, bubbletea.Batch(
			m.detail.spinner.Tick,
			createPR(m.cfg, m.detail.branch),
		)

	case submitResultMsg:
		m.detail.executing = false
		m.detail.executingLabel = ""
		if m.detail.submit != nil {
			m.detail.submit = nil
		}
		if msg.err != nil {
			m.detail.result = styleError.Render("Error: " + msg.err.Error())
			m.detail.refreshViewport()
			return m, nil
		}
		m.detail.prURL = msg.prURL
		m.detail.result = styleSuccess.Render("PR created: " + msg.prURL)
		m.detail.refreshViewport()
		return m, nil

	case settingsSavedMsg:
		if msg.err != nil {
			m.settings.result = styleError.Render("Error: " + msg.err.Error())
			return m, nil
		}
		m.cfg.Mode = msg.mode
		m.cfg.Signing = msg.signing
		m.detail.mode = msg.mode
		m.settings.mode = msg.mode
		m.settings.signing = msg.signing
		m.settings.result = styleSuccess.Render("Saved")
		return m, nil

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
	case viewMe:
		m.me, cmd = m.me.update(msg)
	case viewSettings:
		m.settings, cmd = m.settings.update(msg, m.cfg)
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
		hints = "j/k: navigate  enter: open  s/t/p/o: filters  i: mine  P: project  /: search  m: me  S: settings  q: quit"
	case viewDetail:
		content = m.detail.view()
		hints = "esc: back  j/k: scroll  c/u/x/X/D: actions  q: quit"
	case viewMe:
		content = m.me.view()
		hints = "j/k: navigate  enter: open  esc: back  S: settings  q: quit"
	case viewSettings:
		content = m.settings.view(m.cfg)
		hints = "j/k: select  enter: toggle  esc: back  q: quit"
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

// sdkDetailToMsg converts an SDK DetailResult to a TUI detailDataMsg.
func sdkDetailToMsg(d *sdk.DetailResult) detailDataMsg {
	return detailDataMsg{
		item:          d.Item,
		completion:    d.Completion,
		stamp:         d.Stamp,
		branch:        d.Branch,
		mainStatus:    d.MainStatus,
		prURL:         d.PRURL,
		branchActions: d.BranchActions,
	}
}

func fetchBrowse(cfg Config, f commons.BrowseFilter) bubbletea.Cmd {
	return func() bubbletea.Msg {
		result, err := cfg.Client.Browse(f)
		if err != nil {
			return browseDataMsg{err: err}
		}
		return browseDataMsg{items: result.Items, branchIDs: result.BranchIDs}
	}
}

func fetchMe(cfg Config) bubbletea.Cmd {
	return func() bubbletea.Msg {
		data, err := cfg.Client.Dashboard()
		return meDataMsg{data: data, err: err}
	}
}

func fetchDetail(cfg Config, wantedID string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		result, err := cfg.Client.Detail(wantedID)
		if err != nil {
			return detailDataMsg{err: err}
		}
		return sdkDetailToMsg(result)
	}
}

func executeMutation(cfg Config, wantedID string, t commons.Transition) bubbletea.Cmd {
	return func() bubbletea.Msg {
		var result *sdk.MutationResult
		var err error
		switch t {
		case commons.TransitionClaim:
			result, err = cfg.Client.Claim(wantedID)
		case commons.TransitionUnclaim:
			result, err = cfg.Client.Unclaim(wantedID)
		case commons.TransitionReject:
			result, err = cfg.Client.Reject(wantedID, "")
		case commons.TransitionClose:
			result, err = cfg.Client.Close(wantedID)
		case commons.TransitionDelete:
			result, err = cfg.Client.Delete(wantedID)
		default:
			err = fmt.Errorf("unsupported transition")
		}
		return actionResultMsg{err: err, result: result}
	}
}

func executeDelta(cfg Config, branch string, action branchDeltaAction) bubbletea.Cmd {
	return func() bubbletea.Msg {
		var err error
		switch action {
		case deltaApply:
			err = cfg.Client.ApplyBranch(branch)
		case deltaDiscard:
			err = cfg.Client.DiscardBranch(branch)
		default:
			return deltaResultMsg{err: fmt.Errorf("unknown delta action")}
		}
		if err != nil {
			return deltaResultMsg{err: err}
		}
		switch action {
		case deltaApply:
			return deltaResultMsg{hint: "applied"}
		default:
			return deltaResultMsg{hint: "discarded"}
		}
	}
}

func executeDoneMutation(cfg Config, wantedID, evidence string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		result, err := cfg.Client.Done(wantedID, evidence)
		return actionResultMsg{err: err, result: result}
	}
}

func executeAcceptMutation(cfg Config, wantedID string, msg acceptSubmitMsg) bubbletea.Cmd {
	return func() bubbletea.Msg {
		result, err := cfg.Client.Accept(wantedID, sdk.AcceptInput{
			Quality:     msg.quality,
			Reliability: msg.reliability,
			Severity:    msg.severity,
			SkillTags:   msg.skills,
			Message:     msg.message,
		})
		return actionResultMsg{err: err, result: result}
	}
}

func fetchDiff(cfg Config, branch string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		diff, err := cfg.Client.BranchDiff(branch)
		return submitDiffMsg{diff: diff, err: err}
	}
}

func createPR(cfg Config, branch string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		prURL, err := cfg.Client.SubmitPR(branch)
		return submitResultMsg{prURL: prURL, err: err}
	}
}
