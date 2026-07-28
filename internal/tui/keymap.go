package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Top        key.Binding
	Bottom     key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Toggle     key.Binding
	ToggleAll  key.Binding
	All        key.Binding
	None       key.Binding
	Confirm    key.Binding
	Enter      key.Binding
	Help       key.Binding
	Back       key.Binding
	Refresh    key.Binding
	SortNext   key.Binding
	SortPrev   key.Binding
	SortToggle key.Binding
	Filter     key.Binding
	Quit       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "bottom"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+b"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+f"),
			key.WithHelp("pgdown", "page down"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		ToggleAll: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "toggle all"),
		),
		All: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		None: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "deselect all"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "cleanup"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace", "delete", "ctrl+h"),
			key.WithHelp("backspace", "back"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		SortNext: key.NewBinding(
			key.WithKeys(">"),
			key.WithHelp(">", "next sort column"),
		),
		SortPrev: key.NewBinding(
			key.WithKeys("<"),
			key.WithHelp("<", "previous sort column"),
		),
		SortToggle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle sort direction"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
