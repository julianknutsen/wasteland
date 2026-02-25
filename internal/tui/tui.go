package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

// Config holds the parameters needed to launch the TUI.
type Config struct {
	DB        commons.DB // database backend
	RigHandle string     // current rig handle
	Upstream  string     // upstream identifier for display
	Mode      string     // "wild-west" or "pr"
	Signing   bool       // GPG-signed dolt commits
	HopURI    string     // rig's HOP protocol URI for done/accept dolt commits

	// Settings view: read-only context
	ProviderType string
	ForkOrg      string
	ForkDB       string
	LocalDir     string
	JoinedAt     string

	// Settings persistence (nil = settings read-only)
	SaveConfig func(mode string, signing bool) error

	// PR submission callbacks (nil = feature disabled)
	LoadDiff func(branch string) (string, error)
	CreatePR func(branch string) (prURL string, err error)
	CheckPR  func(branch string) (prURL string) // returns existing PR URL or ""
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
		if msg.detail != nil {
			// PR mode: if the branch status now matches main, the delta
			// is a no-op (e.g. claim then unclaim). Auto-cleanup the branch
			// which also closes any upstream PR (providers close PRs when
			// the source branch is deleted).
			if msg.detail.mainStatus != "" && msg.detail.item != nil &&
				msg.detail.item.Status == msg.detail.mainStatus {
				branch := msg.hint
				return m, func() bubbletea.Msg {
					_ = m.cfg.DB.DeleteBranch(branch)
					_ = m.cfg.DB.DeleteRemoteBranch(branch)
					return deltaResultMsg{hint: "discarded"}
				}
			}
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
		completionID := ""
		if m.detail.completion != nil {
			completionID = m.detail.completion.ID
		}
		return m, bubbletea.Batch(
			m.detail.spinner.Tick,
			executeAcceptMutation(m.cfg, m.detail.item.ID, msg, completionID),
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

func fetchBrowse(cfg Config, f commons.BrowseFilter) bubbletea.Cmd {
	return func() bubbletea.Msg {
		items, branchIDs, err := commons.BrowseWantedBranchAware(cfg.DB, cfg.Mode, cfg.RigHandle, f)
		if err != nil {
			return browseDataMsg{err: err}
		}
		return browseDataMsg{items: items, branchIDs: branchIDs}
	}
}

func fetchMe(cfg Config) bubbletea.Cmd {
	return func() bubbletea.Msg {
		data, err := commons.QueryMyDashboardBranchAware(cfg.DB, cfg.Mode, cfg.RigHandle)
		return meDataMsg{data: data, err: err}
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
	err := cfg.DB.PushWithSync(io.Discard)
	return actionResultMsg{err: err}
}

// mutateOnBranch is the common pattern for all PR-mode mutations:
// execute DML on branch → read detail → push.
func mutateOnBranch(cfg Config, wantedID, commitMsg string, stmts ...string) actionResultMsg {
	branch := commons.BranchName(cfg.RigHandle, wantedID)
	mainStatus, _, _ := commons.QueryItemStatus(cfg.DB, wantedID, "main")

	if err := cfg.DB.Exec(branch, commitMsg, cfg.Signing, stmts...); err != nil {
		return actionResultMsg{err: err}
	}

	// Read post-mutation state from branch.
	item, completion, stamp, _ := commons.QueryFullDetailAsOf(cfg.DB, wantedID, branch)
	delta := ""
	if mainStatus != "" && item != nil && item.Status != mainStatus {
		delta = commons.DeltaLabel(mainStatus, item.Status)
	}
	var prURL string
	if branch != "" && cfg.CheckPR != nil {
		prURL = cfg.CheckPR(branch)
	}
	detail := detailDataMsg{
		item: item, completion: completion, stamp: stamp,
		branch: branch, mainStatus: mainStatus, prURL: prURL,
		branchActions: sdk.ComputeBranchActions(cfg.Mode, branch, delta, prURL),
	}

	var pushLog bytes.Buffer
	if err := cfg.DB.PushBranch(branch, &pushLog); err != nil {
		if msg := strings.TrimSpace(pushLog.String()); msg != "" {
			return actionResultMsg{err: fmt.Errorf("%s", msg)}
		}
		return actionResultMsg{err: fmt.Errorf("push branch: %w", err)}
	}

	return actionResultMsg{hint: branch, detail: &detail}
}

// transitionDML returns the DML statements and commit message for a lifecycle transition.
func transitionDML(cfg Config, wantedID string, t commons.Transition) ([]string, string) {
	switch t {
	case commons.TransitionClaim:
		return []string{commons.ClaimWantedDML(wantedID, cfg.RigHandle)}, "wl claim: " + wantedID
	case commons.TransitionUnclaim:
		return []string{commons.UnclaimWantedDML(wantedID)}, "wl unclaim: " + wantedID
	case commons.TransitionReject:
		return commons.RejectCompletionDML(wantedID), "wl reject: " + wantedID
	case commons.TransitionClose:
		return []string{commons.CloseWantedDML(wantedID)}, "wl close: " + wantedID
	case commons.TransitionDelete:
		return []string{commons.DeleteWantedDML(wantedID)}, "wl delete: " + wantedID
	default:
		return nil, ""
	}
}

func executePRMutation(cfg Config, wantedID string, t commons.Transition) actionResultMsg {
	stmts, msg := transitionDML(cfg, wantedID, t)
	if stmts == nil {
		return actionResultMsg{err: fmt.Errorf("unsupported transition")}
	}
	return mutateOnBranch(cfg, wantedID, msg, stmts...)
}

func applyTransition(cfg Config, wantedID string, t commons.Transition) error {
	switch t {
	case commons.TransitionClaim:
		return commons.ClaimWanted(cfg.DB, wantedID, cfg.RigHandle, cfg.Signing)
	case commons.TransitionUnclaim:
		return commons.UnclaimWanted(cfg.DB, wantedID, cfg.Signing)
	case commons.TransitionReject:
		return commons.RejectCompletion(cfg.DB, wantedID, "", "", cfg.Signing)
	case commons.TransitionClose:
		return commons.CloseWanted(cfg.DB, wantedID, cfg.Signing)
	case commons.TransitionDelete:
		return commons.DeleteWanted(cfg.DB, wantedID, cfg.Signing)
	default:
		return fmt.Errorf("unsupported transition")
	}
}

func fetchDetail(cfg Config, wantedID string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		if cfg.Mode == "pr" {
			state, _ := commons.ResolveItemState(cfg.DB, cfg.RigHandle, wantedID)
			if state != nil && state.Effective() != nil {
				detail := detailDataMsg{
					item:       state.Effective(),
					completion: state.Completion,
					stamp:      state.Stamp,
					branch:     state.BranchName,
				}
				if state.Main != nil {
					detail.mainStatus = state.Main.Status
				}
				if state.BranchName != "" && cfg.CheckPR != nil {
					detail.prURL = cfg.CheckPR(state.BranchName)
				}
				detail.branchActions = sdk.ComputeBranchActions(
					cfg.Mode, detail.branch, state.Delta(), detail.prURL,
				)
				return detail
			}
		}
		return fetchDetailSync(cfg.DB, wantedID)
	}
}

func executeDelta(cfg Config, branch string, action branchDeltaAction) bubbletea.Cmd {
	return func() bubbletea.Msg {
		switch action {
		case deltaApply:
			return executeDeltaApply(cfg, branch)
		case deltaDiscard:
			return executeDeltaDiscard(cfg, branch)
		default:
			return deltaResultMsg{err: fmt.Errorf("unknown delta action")}
		}
	}
}

func executeDeltaApply(cfg Config, branch string) deltaResultMsg {
	// Merge branch into main.
	if err := cfg.DB.MergeBranch(branch); err != nil {
		return deltaResultMsg{err: err}
	}
	// Delete local branch.
	if err := cfg.DB.DeleteBranch(branch); err != nil {
		return deltaResultMsg{err: fmt.Errorf("delete local branch: %w", err)}
	}
	// Push merged main to origin.
	if err := cfg.DB.PushMain(io.Discard); err != nil {
		return deltaResultMsg{err: fmt.Errorf("push origin main: %w", err)}
	}
	return deltaResultMsg{hint: "applied"}
}

func executeDeltaDiscard(cfg Config, branch string) deltaResultMsg {
	// Delete local branch.
	if err := cfg.DB.DeleteBranch(branch); err != nil {
		return deltaResultMsg{err: fmt.Errorf("delete local branch: %w", err)}
	}
	// Delete remote branch (best-effort).
	_ = cfg.DB.DeleteRemoteBranch(branch)
	return deltaResultMsg{hint: "discarded"}
}

// --- done/accept mutation commands ---

func applyDone(cfg Config, wantedID, evidence string) error {
	completionID := commons.GeneratePrefixedID("c", wantedID, cfg.RigHandle)
	return commons.SubmitCompletion(cfg.DB, completionID, wantedID,
		cfg.RigHandle, evidence, cfg.HopURI, cfg.Signing)
}

func applyAccept(cfg Config, wantedID string, msg acceptSubmitMsg, completionID string) error {
	stamp := &commons.Stamp{
		ID:          commons.GeneratePrefixedID("s", wantedID, cfg.RigHandle),
		Author:      cfg.RigHandle,
		Subject:     completionID,
		Quality:     msg.quality,
		Reliability: msg.reliability,
		Severity:    msg.severity,
		ContextID:   completionID,
		ContextType: "completion",
		SkillTags:   msg.skills,
		Message:     msg.message,
	}
	return commons.AcceptCompletion(cfg.DB, wantedID, completionID,
		cfg.RigHandle, cfg.HopURI, stamp, cfg.Signing)
}

func executeDoneMutation(cfg Config, wantedID, evidence string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		if cfg.Mode == "pr" {
			return executePRDoneMutation(cfg, wantedID, evidence)
		}
		return executeWildWestDoneMutation(cfg, wantedID, evidence)
	}
}

func executeWildWestDoneMutation(cfg Config, wantedID, evidence string) actionResultMsg {
	if err := applyDone(cfg, wantedID, evidence); err != nil {
		return actionResultMsg{err: err}
	}
	err := cfg.DB.PushWithSync(io.Discard)
	return actionResultMsg{err: err}
}

func executePRDoneMutation(cfg Config, wantedID, evidence string) actionResultMsg {
	completionID := commons.GeneratePrefixedID("c", wantedID, cfg.RigHandle)
	stmts := commons.SubmitCompletionDML(completionID, wantedID, cfg.RigHandle, evidence, cfg.HopURI)
	return mutateOnBranch(cfg, wantedID, "wl done: "+wantedID, stmts...)
}

func executeAcceptMutation(cfg Config, wantedID string, msg acceptSubmitMsg, completionID string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		if cfg.Mode == "pr" {
			return executePRAcceptMutation(cfg, wantedID, msg, completionID)
		}
		return executeWildWestAcceptMutation(cfg, wantedID, msg, completionID)
	}
}

func executeWildWestAcceptMutation(cfg Config, wantedID string, msg acceptSubmitMsg, completionID string) actionResultMsg {
	if err := applyAccept(cfg, wantedID, msg, completionID); err != nil {
		return actionResultMsg{err: err}
	}
	err := cfg.DB.PushWithSync(io.Discard)
	return actionResultMsg{err: err}
}

func executePRAcceptMutation(cfg Config, wantedID string, msg acceptSubmitMsg, completionID string) actionResultMsg {
	stamp := &commons.Stamp{
		ID:          commons.GeneratePrefixedID("s", wantedID, cfg.RigHandle),
		Author:      cfg.RigHandle,
		Subject:     completionID,
		Quality:     msg.quality,
		Reliability: msg.reliability,
		Severity:    msg.severity,
		ContextID:   completionID,
		ContextType: "completion",
		SkillTags:   msg.skills,
		Message:     msg.message,
	}
	stmts := commons.AcceptCompletionDML(wantedID, completionID, cfg.RigHandle, cfg.HopURI, stamp)
	return mutateOnBranch(cfg, wantedID, "wl accept: "+wantedID, stmts...)
}

// --- submit PR commands ---

func fetchDiff(cfg Config, branch string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		if cfg.LoadDiff == nil {
			return submitDiffMsg{err: fmt.Errorf("diff loading not available")}
		}
		diff, err := cfg.LoadDiff(branch)
		return submitDiffMsg{diff: diff, err: err}
	}
}

func createPR(cfg Config, branch string) bubbletea.Cmd {
	return func() bubbletea.Msg {
		if cfg.CreatePR == nil {
			return submitResultMsg{err: fmt.Errorf("PR creation not available")}
		}
		prURL, err := cfg.CreatePR(branch)
		return submitResultMsg{prURL: prURL, err: err}
	}
}

// fetchDetailSync queries detail data from the working copy (main).
func fetchDetailSync(db commons.DB, wantedID string) detailDataMsg {
	item, completion, stamp, err := commons.QueryFullDetail(db, wantedID)
	if err != nil {
		return detailDataMsg{err: err}
	}
	return detailDataMsg{item: item, completion: completion, stamp: stamp}
}
