package tui

import (
	"math/rand"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zpdzap/sandcastles/internal/config"
	"github.com/zpdzap/sandcastles/internal/sandbox"
	"golang.org/x/term"
)

// model is the Bubble Tea model for the sandcastles TUI.
type model struct {
	manager    *sandbox.Manager
	cfg        *config.Config
	input      textinput.Model
	cursor     int
	message    string
	isError    bool
	commanding bool // true when in command mode (/ pressed)
	quitting   bool
	connectTo  string // sandbox name to connect to after tea quits
	width      int
	height     int
	progressName  string  // name of sandbox being created
	progressPhase *string // current phase (shared pointer so background goroutine updates are visible)
	quip          string  // random phrase shown in header, constant per session
}

func newModel(mgr *sandbox.Manager, cfg *config.Config) model {
	ti := textinput.New()
	ti.Placeholder = "start, stop, connect, diff, merge <name> | quit"
	ti.CharLimit = 256
	ti.Width = 80
	// Input starts unfocused — activated by pressing /
	ti.Blur()

	// Get initial terminal size so the first render isn't at width=0
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	return model{
		manager: mgr,
		cfg:     cfg,
		input:   ti,
		width:   w,
		height:  h,
		quip:    quips[rand.Intn(len(quips))],
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}
