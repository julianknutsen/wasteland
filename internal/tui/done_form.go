package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	bubbletea "github.com/charmbracelet/bubbletea"
)

type doneFormModel struct {
	evidence textinput.Model
	active   bool
	err      string // validation error
}

func newDoneForm() *doneFormModel {
	ti := textinput.New()
	ti.Placeholder = "https://github.com/org/repo/pull/123"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 60
	return &doneFormModel{
		evidence: ti,
		active:   true,
	}
}

func (m *doneFormModel) update(msg bubbletea.Msg) (*doneFormModel, bubbletea.Cmd) {
	if msg, ok := msg.(bubbletea.KeyMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			// Cancel form.
			return nil, nil
		case msg.Type == bubbletea.KeyEnter:
			val := m.evidence.Value()
			if val == "" {
				m.err = "evidence URL is required"
				return m, nil
			}
			m.err = ""
			return m, func() bubbletea.Msg {
				return doneSubmitMsg{evidence: val}
			}
		}
	}

	var cmd bubbletea.Cmd
	m.evidence, cmd = m.evidence.Update(msg)
	return m, cmd
}

func (m *doneFormModel) view() string {
	var s string
	s += styleConfirm.Render("  Done: submit completion evidence") + "\n"
	s += "  Evidence: " + m.evidence.View() + "\n"
	if m.err != "" {
		s += "  " + styleError.Render(m.err) + "\n"
	}
	s += styleDim.Render("  enter: submit   esc: cancel") + "\n"
	return s
}
