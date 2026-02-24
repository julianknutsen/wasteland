package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
)

// confirmAction holds state while waiting for the user to confirm.
type confirmAction struct {
	transition commons.Transition
	label      string
}

type detailModel struct {
	item       *commons.WantedItem
	completion *commons.CompletionRecord
	stamp      *commons.Stamp
	viewport   viewport.Model
	width      int
	height     int
	loading    bool
	err        error

	// Mutation state.
	dbDir          string
	rigHandle      string
	mode           string
	branch         string         // non-empty when showing branch state
	confirming     *confirmAction // non-nil → showing confirmation prompt
	executing      bool           // true → showing spinner
	executingLabel string         // e.g. "Claiming..."
	spinner        spinner.Model
	result         string // brief success/error message
}

func newDetailModel(dbDir, rigHandle, mode string) detailModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	return detailModel{
		dbDir:     dbDir,
		rigHandle: rigHandle,
		mode:      mode,
		spinner:   s,
		loading:   true,
	}
}

func (m *detailModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h - 2 // room for title + padding
}

func (m *detailModel) refreshViewport() {
	if m.item != nil {
		m.viewport.SetContent(m.renderContent())
	}
}

func (m *detailModel) setData(msg detailDataMsg) {
	m.loading = false
	m.err = msg.err
	m.item = msg.item
	m.completion = msg.completion
	m.stamp = msg.stamp
	m.branch = msg.branch
	// Clear mutation state so stale results don't mask action hints.
	m.confirming = nil
	m.executing = false
	m.executingLabel = ""
	m.result = ""
	if m.item != nil {
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
	}
}

func (m detailModel) update(msg bubbletea.Msg) (detailModel, bubbletea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.executing {
			var cmd bubbletea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case bubbletea.KeyMsg:
		// While executing, only allow ctrl+c.
		if m.executing {
			return m, nil
		}

		// Confirmation prompt active: handle y/n/esc only.
		if m.confirming != nil {
			switch {
			case key.Matches(msg, keys.Confirm):
				t := m.confirming.transition
				m.confirming = nil
				return m, func() bubbletea.Msg {
					return actionConfirmedMsg{transition: t}
				}
			case key.Matches(msg, keys.Cancel), key.Matches(msg, keys.Back):
				m.confirming = nil
				return m, nil
			}
			return m, nil
		}

		// Normal key handling.
		switch {
		case key.Matches(msg, keys.Back):
			return m, func() bubbletea.Msg {
				return navigateMsg{view: viewBrowse}
			}
		case key.Matches(msg, keys.Quit):
			return m, bubbletea.Quit

		// Actions requiring text input — show CLI hint.
		case key.Matches(msg, keys.Done):
			return m.tryTextAction(commons.TransitionDone)
		case key.Matches(msg, keys.Accept):
			return m.tryTextAction(commons.TransitionAccept)

		// Executable actions.
		case key.Matches(msg, keys.Claim):
			return m.tryAction(commons.TransitionClaim)
		case key.Matches(msg, keys.Unclaim):
			return m.tryAction(commons.TransitionUnclaim)
		case key.Matches(msg, keys.Reject):
			return m.tryAction(commons.TransitionReject)
		case key.Matches(msg, keys.Close):
			return m.tryAction(commons.TransitionClose)
		case key.Matches(msg, keys.Delete):
			return m.tryAction(commons.TransitionDelete)
		}
	}

	var cmd bubbletea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// tryAction validates a transition and permission, then returns an actionRequestMsg.
func (m detailModel) tryAction(t commons.Transition) (detailModel, bubbletea.Cmd) {
	if m.item == nil {
		return m, nil
	}
	if _, err := commons.ValidateTransition(m.item.Status, t); err != nil {
		m.result = styleError.Render(err.Error())
		m.viewport.SetContent(m.renderContent())
		return m, nil
	}
	if !commons.CanPerformTransition(m.item, t, m.rigHandle) {
		name := commons.TransitionName(t)
		m.result = styleError.Render(fmt.Sprintf("cannot %s: permission denied", name))
		m.viewport.SetContent(m.renderContent())
		return m, nil
	}
	name := commons.TransitionName(t)
	verb := strings.ToUpper(name[:1]) + name[1:]
	label := fmt.Sprintf("%s %s?", verb, m.item.ID)
	return m, func() bubbletea.Msg {
		return actionRequestMsg{transition: t, label: label}
	}
}

// tryTextAction handles transitions that require additional CLI input.
func (m detailModel) tryTextAction(t commons.Transition) (detailModel, bubbletea.Cmd) {
	if m.item == nil {
		return m, nil
	}
	if _, err := commons.ValidateTransition(m.item.Status, t); err == nil {
		if commons.CanPerformTransition(m.item, t, m.rigHandle) {
			name := commons.TransitionName(t)
			hint := commons.TransitionRequiresInput(t)
			m.result = styleDim.Render(fmt.Sprintf("use `wl %s %s` — %s", name, m.item.ID, hint))
			m.viewport.SetContent(m.renderContent())
		}
	}
	return m, nil
}

func (m detailModel) view() string {
	if m.loading {
		return styleDim.Render("  Loading...")
	}
	if m.err != nil {
		return fmt.Sprintf("  Error: %v", m.err)
	}
	if m.item == nil {
		return styleDim.Render("  No item loaded.")
	}

	title := styleTitle.Render(fmt.Sprintf("%s: %s", m.item.ID, m.item.Title))
	return title + "\n" + m.viewport.View()
}

func (m detailModel) renderContent() string {
	item := m.item
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  Status:      %s\n", colorizeStatus(item.Status)))
	if m.branch != "" {
		b.WriteString(styleDim.Render(fmt.Sprintf("  Branch:      %s\n", m.branch)))
	}

	if item.Type != "" {
		b.WriteString(fmt.Sprintf("  Type:        %-14s", item.Type))
	} else {
		b.WriteString(fmt.Sprintf("  %-28s", ""))
	}
	b.WriteString(fmt.Sprintf("Priority: %s\n", colorizePriority(item.Priority)))

	if item.Project != "" {
		b.WriteString(fmt.Sprintf("  Project:     %-14s", item.Project))
	} else {
		b.WriteString(fmt.Sprintf("  %-28s", ""))
	}
	b.WriteString(fmt.Sprintf("Effort:   %s\n", item.EffortLevel))

	if item.PostedBy != "" {
		b.WriteString(fmt.Sprintf("  Posted by:   %s\n", item.PostedBy))
	}

	if item.ClaimedBy != "" {
		b.WriteString(fmt.Sprintf("  Claimed by:  %s\n", item.ClaimedBy))
	}

	if len(item.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  Tags:        %s\n", strings.Join(item.Tags, ", ")))
	}

	if item.CreatedAt != "" {
		b.WriteString(fmt.Sprintf("  Created:     %s\n", item.CreatedAt))
	}
	if item.UpdatedAt != "" {
		b.WriteString(fmt.Sprintf("  Updated:     %s\n", item.UpdatedAt))
	}

	if item.Description != "" {
		b.WriteString("\n  Description:\n")
		b.WriteString(fmt.Sprintf("    %s\n", item.Description))
	}

	if m.completion != nil {
		b.WriteString(fmt.Sprintf("\n  Completion:  %s\n", m.completion.ID))
		if m.completion.Evidence != "" {
			b.WriteString(fmt.Sprintf("    Evidence:    %s\n", m.completion.Evidence))
		}
		b.WriteString(fmt.Sprintf("    Completed by: %s\n", m.completion.CompletedBy))
	}

	if m.stamp != nil {
		b.WriteString(fmt.Sprintf("\n  Stamp:       %s\n", m.stamp.ID))
		b.WriteString(fmt.Sprintf("    Quality: %d  Reliability: %d  Severity: %s\n",
			m.stamp.Quality, m.stamp.Reliability, m.stamp.Severity))
		if len(m.stamp.SkillTags) > 0 {
			b.WriteString(fmt.Sprintf("    Skills:      %s\n", strings.Join(m.stamp.SkillTags, ", ")))
		}
		if m.stamp.Author != "" {
			b.WriteString(fmt.Sprintf("    Accepted by: %s\n", m.stamp.Author))
		}
		if m.stamp.Message != "" {
			b.WriteString(fmt.Sprintf("    Message:     %s\n", m.stamp.Message))
		}
	}

	// Status line: confirmation, executing, result, or action hints.
	b.WriteByte('\n')
	switch {
	case m.confirming != nil:
		b.WriteString(styleConfirm.Render(fmt.Sprintf(
			"  %s Pushes to upstream. [y/n]", m.confirming.label)))
	case m.executing:
		b.WriteString(fmt.Sprintf("  %s %s", m.spinner.View(), m.executingLabel))
	case m.result != "":
		b.WriteString("  " + m.result)
	default:
		b.WriteString(styleDim.Render(m.actionHints()))
	}
	b.WriteByte('\n')

	return b.String()
}

// transitionKeyHint maps transitions to their TUI key bindings.
var transitionKeyHint = map[commons.Transition]string{
	commons.TransitionClaim:   "c",
	commons.TransitionUnclaim: "u",
	commons.TransitionDone:    "d",
	commons.TransitionAccept:  "a",
	commons.TransitionReject:  "x",
	commons.TransitionClose:   "X",
	commons.TransitionDelete:  "D",
}

// actionHints returns a string showing valid lifecycle actions for the item,
// filtered by both status validity and permission.
func (m detailModel) actionHints() string {
	if m.item == nil {
		return ""
	}
	available := commons.AvailableTransitions(m.item, m.rigHandle)
	if len(available) == 0 {
		return "  (no actions available)"
	}
	var hints []string
	for _, t := range available {
		k := transitionKeyHint[t]
		name := commons.TransitionName(t)
		hint := k + ":" + name
		if commons.TransitionRequiresInput(t) != "" {
			hint += "*"
		}
		hints = append(hints, hint)
	}
	return "  Actions: " + strings.Join(hints, "  ")
}
