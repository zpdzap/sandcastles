package tui

import (
	"math/rand"
	"os"
	"time"

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
	messageID  int
	commanding bool // true when in command mode (/ pressed)
	quitting   bool
	attaching  bool // suppress final render before ExecProcess handoff
	width      int
	height     int
	progressName  string  // name of sandbox being created
	progressPhase *string // current phase (shared pointer so background goroutine updates are visible)
	quip          string  // random phrase shown in header, constant per session

	// Split-pane preview
	previews    map[string]string // cached tmux output per sandbox name
	agentStates map[string]string // "working" / "waiting" / "done" per sandbox
	attachedAt  map[string]time.Time // last time a client was detected attached

	// Diff stats shown in column headers
	diffStats map[string]diffStat // per-sandbox diff summary

	// Modals
	showHelp    bool
	showDiff    bool
	diffContent string // rendered diff tree

	// Double-press stop confirmation
	confirmStop     bool
	confirmStopName string
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

	m := model{
		manager:     mgr,
		cfg:         cfg,
		input:       ti,
		width:       w,
		height:      h,
		quip:        quips[rand.Intn(len(quips))],
		previews:    make(map[string]string),
		agentStates: make(map[string]string),
		diffStats:   make(map[string]diffStat),
		attachedAt:  make(map[string]time.Time),
	}

	return m
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}
