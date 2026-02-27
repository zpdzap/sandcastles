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

	statusRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	statusStopped = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	statusOther   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
)
