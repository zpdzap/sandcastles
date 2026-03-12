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

// confirmStopExpiredMsg cancels a pending stop confirmation.
type confirmStopExpiredMsg struct{}

// clearMessageMsg clears the status message if its ID still matches.
type clearMessageMsg struct {
	id int
}

// attachFinishedMsg is sent when the user detaches from a tmux session.
type attachFinishedMsg struct {
	name string
}

// statusPollResultMsg carries the results of async status polling.
type statusPollResultMsg struct {
	previews    map[string]string
	agentStates map[string]string
	diffStats   map[string]diffStat
	attachedAt  map[string]time.Time
}

// tickCmd returns a command that sends a tick every 2 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return statusTickMsg(t)
	})
}
