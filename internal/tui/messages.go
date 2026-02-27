package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// sandboxCreatedMsg is sent when a sandbox finishes creating.
type sandboxCreatedMsg struct {
	name string
	err  error
}

// sandboxDestroyedMsg is sent when a sandbox is destroyed.
type sandboxDestroyedMsg struct {
	name string
}

// allDestroyedMsg is sent when all sandboxes have been destroyed.
type allDestroyedMsg struct {
	count int
}

// statusTickMsg triggers a status refresh poll.
type statusTickMsg time.Time

// tickCmd returns a command that sends a tick every 2 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return statusTickMsg(t)
	})
}
