package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zpdzap/sandcastles/internal/config"
	"github.com/zpdzap/sandcastles/internal/sandbox"
)

// model is the Bubble Tea model for the sandcastles TUI.
type model struct {
	manager   *sandbox.Manager
	cfg       *config.Config
	input     textinput.Model
	cursor    int
	message   string
	isError   bool
	quitting  bool
	connectTo string // sandbox name to connect to after tea quits
	width     int
	height    int
}

func newModel(mgr *sandbox.Manager, cfg *config.Config) model {
	ti := textinput.New()
	ti.Placeholder = "/start <name> [task], /stop <name>, /connect <name>, /diff <name>, /quit"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	return model{
		manager: mgr,
		cfg:     cfg,
		input:   ti,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd())
}
