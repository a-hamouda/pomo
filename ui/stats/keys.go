package stats

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	PreviousPeriod key.Binding
	NextPeriod     key.Binding
	PreviousTask   key.Binding
	NextTask       key.Binding
	Today          key.Binding
	Quit           key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.PreviousPeriod,
		k.PreviousTask,
		k.Today,
		k.Quit,
	}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var Keys = KeyMap{
	PreviousPeriod: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/→", "week"),
	),
	NextPeriod: key.NewBinding(
		key.WithKeys("right", "l"),
	),
	PreviousTask: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/↓", "task"),
	),
	NextTask: key.NewBinding(
		key.WithKeys("down", "j"),
	),
	Today: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "today"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "quit"),
	),
}
