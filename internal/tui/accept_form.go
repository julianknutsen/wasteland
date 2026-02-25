package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	bubbletea "github.com/charmbracelet/bubbletea"
)

var severityOptions = []string{"leaf", "branch", "root"}

type acceptFormModel struct {
	quality     textinput.Model // "1"-"5"
	reliability textinput.Model // "1"-"5", optional
	severityIdx int             // index into severityOptions
	skills      textinput.Model
	message     textinput.Model
	cursor      int // 0-4: quality, reliability, severity, skills, message
	active      bool
	err         string
}

func newAcceptForm() *acceptFormModel {
	quality := textinput.New()
	quality.Placeholder = "1-5"
	quality.Focus()
	quality.CharLimit = 1
	quality.Width = 10

	reliability := textinput.New()
	reliability.Placeholder = "1-5 (defaults to quality)"
	reliability.CharLimit = 1
	reliability.Width = 30

	skills := textinput.New()
	skills.Placeholder = "go, dolt"
	skills.CharLimit = 200
	skills.Width = 40

	message := textinput.New()
	message.Placeholder = "optional review message"
	message.CharLimit = 500
	message.Width = 50

	return &acceptFormModel{
		quality:     quality,
		reliability: reliability,
		severityIdx: 0,
		skills:      skills,
		message:     message,
		cursor:      0,
		active:      true,
	}
}

func (m *acceptFormModel) focusCurrent() {
	m.quality.Blur()
	m.reliability.Blur()
	m.skills.Blur()
	m.message.Blur()

	switch m.cursor {
	case 0:
		m.quality.Focus()
	case 1:
		m.reliability.Focus()
	case 3:
		m.skills.Focus()
	case 4:
		m.message.Focus()
	}
}

func (m *acceptFormModel) update(msg bubbletea.Msg) (*acceptFormModel, bubbletea.Cmd) {
	if msg, ok := msg.(bubbletea.KeyMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			return nil, nil

		case msg.Type == bubbletea.KeyEnter:
			return m, m.submit()

		case msg.Type == bubbletea.KeyTab, msg.Type == bubbletea.KeyDown:
			m.cursor = (m.cursor + 1) % 5
			m.focusCurrent()
			m.err = ""
			return m, nil

		case msg.Type == bubbletea.KeyShiftTab, msg.Type == bubbletea.KeyUp:
			m.cursor = (m.cursor + 4) % 5
			m.focusCurrent()
			m.err = ""
			return m, nil

		case msg.String() == "j" && m.cursor == 2:
			m.cursor = (m.cursor + 1) % 5
			m.focusCurrent()
			return m, nil

		case msg.String() == "k" && m.cursor == 2:
			m.cursor = (m.cursor + 4) % 5
			m.focusCurrent()
			return m, nil

		case msg.Type == bubbletea.KeyLeft && m.cursor == 2:
			m.severityIdx = (m.severityIdx + len(severityOptions) - 1) % len(severityOptions)
			return m, nil

		case msg.Type == bubbletea.KeyRight && m.cursor == 2:
			m.severityIdx = (m.severityIdx + 1) % len(severityOptions)
			return m, nil
		}
	}

	// Pass through to the active text input.
	var cmd bubbletea.Cmd
	switch m.cursor {
	case 0:
		m.quality, cmd = m.quality.Update(msg)
	case 1:
		m.reliability, cmd = m.reliability.Update(msg)
	case 3:
		m.skills, cmd = m.skills.Update(msg)
	case 4:
		m.message, cmd = m.message.Update(msg)
	}
	return m, cmd
}

func (m *acceptFormModel) submit() bubbletea.Cmd {
	// Validate quality.
	q, err := strconv.Atoi(m.quality.Value())
	if err != nil || q < 1 || q > 5 {
		m.err = "quality must be 1-5"
		return nil
	}

	// Reliability defaults to quality.
	r := q
	if m.reliability.Value() != "" {
		r, err = strconv.Atoi(m.reliability.Value())
		if err != nil || r < 1 || r > 5 {
			m.err = "reliability must be 1-5"
			return nil
		}
	}

	severity := severityOptions[m.severityIdx]

	var skills []string
	if raw := strings.TrimSpace(m.skills.Value()); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				skills = append(skills, s)
			}
		}
	}

	msg := acceptSubmitMsg{
		quality:     q,
		reliability: r,
		severity:    severity,
		skills:      skills,
		message:     strings.TrimSpace(m.message.Value()),
	}
	return func() bubbletea.Msg { return msg }
}

func (m *acceptFormModel) view() string {
	var b strings.Builder

	b.WriteString(styleConfirm.Render("  Accept: issue reputation stamp") + "\n")

	fields := []struct {
		label string
		view  string
	}{
		{"Quality:     ", m.quality.View()},
		{"Reliability: ", m.reliability.View()},
		{"Severity:    ", m.severityView()},
		{"Skills:      ", m.skills.View()},
		{"Message:     ", m.message.View()},
	}

	for i, f := range fields {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		b.WriteString(cursor + f.label + f.view + "\n")
	}

	if m.err != "" {
		b.WriteString("  " + styleError.Render(m.err) + "\n")
	}
	b.WriteString(styleDim.Render("  tab: fields   enter: submit   esc: cancel") + "\n")
	return b.String()
}

func (m *acceptFormModel) severityView() string {
	var parts []string
	for i, opt := range severityOptions {
		if i == m.severityIdx {
			parts = append(parts, "["+opt+"]")
		} else {
			parts = append(parts, " "+opt+" ")
		}
	}
	label := strings.Join(parts, " ")
	if m.cursor == 2 {
		return label + styleDim.Render("  ←/→")
	}
	return label
}
