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

// sandboxProgressMsg is sent during sandbox creation with phase updates.
type sandboxProgressMsg struct {
	name  string
	phase string
}

// sandboxDestroyedMsg is sent when a sandbox is destroyed.
type sandboxDestroyedMsg struct {
	name string
}

// statusTickMsg triggers a status refresh poll.
type statusTickMsg time.Time

// tickCmd returns a command that sends a tick every 2 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return statusTickMsg(t)
	})
}

// listenForProgress returns a tea.Cmd that reads the next progress message from the channel.
// Returns nil when the channel is closed (create finished).
func listenForProgress(ch <-chan sandboxProgressMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}
