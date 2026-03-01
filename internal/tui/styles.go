package tui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			Background(lipgloss.Color("#1a1a2e")).
			Padding(0, 2)

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#333333"))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Padding(1, 2)

	nameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	selectedNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				Bold(true)

	taskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	portStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5599FF"))

	hotkeysStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Padding(0, 2)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Padding(0, 2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Padding(0, 2)

	quipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B7500")).
			Background(lipgloss.Color("#1a1a2e"))

	statusRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	statusStopped = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	statusOther   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))

	// Agent state labels
	stateWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	stateWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	stateDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Preview pane
	previewStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Padding(0, 2)

	previewEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Padding(0, 2)

	// Help modal
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FFD700")).
			Padding(1, 2).
			Foreground(lipgloss.Color("#FFFFFF"))

	helpHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5599FF"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	// Confirmation
	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Padding(0, 2)
)
