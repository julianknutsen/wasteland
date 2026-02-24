package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
)

type detailModel struct {
	item       *commons.WantedItem
	completion *commons.CompletionRecord
	stamp      *commons.Stamp
	viewport   viewport.Model
	width      int
	height     int
	loading    bool
	err        error
}

func newDetailModel() detailModel {
	return detailModel{loading: true}
}

func (m *detailModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h - 2 // room for title + padding
}

func (m *detailModel) setData(msg detailDataMsg) {
	m.loading = false
	m.err = msg.err
	m.item = msg.item
	m.completion = msg.completion
	m.stamp = msg.stamp
	if m.item != nil {
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
	}
}

func (m detailModel) update(msg bubbletea.Msg) (detailModel, bubbletea.Cmd) {
	if msg, ok := msg.(bubbletea.KeyMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			return m, func() bubbletea.Msg {
				return navigateMsg{view: viewBrowse}
			}
		case key.Matches(msg, keys.Quit):
			return m, bubbletea.Quit
		}
	}

	var cmd bubbletea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
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

	// Show valid actions as dimmed hints (Phase 1: display only).
	b.WriteString("\n")
	b.WriteString(styleDim.Render(m.actionHints()))
	b.WriteByte('\n')

	return b.String()
}

// actionHints returns a string showing valid lifecycle actions for the item.
func (m detailModel) actionHints() string {
	if m.item == nil {
		return ""
	}
	var hints []string
	transitions := []struct {
		t    commons.Transition
		hint string
	}{
		{commons.TransitionClaim, "c:claim"},
		{commons.TransitionUnclaim, "u:unclaim"},
		{commons.TransitionDone, "d:done"},
		{commons.TransitionAccept, "a:accept"},
		{commons.TransitionReject, "x:reject"},
		{commons.TransitionClose, "X:close"},
		{commons.TransitionDelete, "D:delete"},
	}
	for _, tr := range transitions {
		if _, err := commons.ValidateTransition(m.item.Status, tr.t); err == nil {
			hints = append(hints, tr.hint)
		}
	}
	if len(hints) == 0 {
		return "  (no actions available)"
	}
	return "  Actions: " + strings.Join(hints, "  ")
}
