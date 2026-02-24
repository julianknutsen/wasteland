package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	Search   key.Binding
	Status   key.Binding
	Type     key.Binding
	Priority key.Binding
	Project  key.Binding
	MyItems  key.Binding
	Sort     key.Binding
	Me       key.Binding
	Claim    key.Binding
	Unclaim  key.Binding
	Done     key.Binding
	Accept   key.Binding
	Reject   key.Binding
	Close    key.Binding
	Delete   key.Binding
	Apply    key.Binding
	Discard  key.Binding
	Confirm  key.Binding
	Cancel   key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("j/down", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Status: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "status"),
	),
	Type: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "type"),
	),
	Priority: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "priority"),
	),
	Project: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "project"),
	),
	MyItems: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "mine"),
	),
	Sort: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "sort"),
	),
	Me: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "me"),
	),
	Claim: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "claim"),
	),
	Unclaim: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "unclaim"),
	),
	Done: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "done"),
	),
	Accept: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "accept"),
	),
	Reject: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "reject"),
	),
	Close: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "close"),
	),
	Delete: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "delete"),
	),
	Apply: key.NewBinding(
		key.WithKeys("M"),
		key.WithHelp("M", "apply"),
	),
	Discard: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "discard"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "confirm"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "cancel"),
	),
}
