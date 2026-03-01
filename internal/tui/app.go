package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zpdzap/sandcastles/internal/config"
	"github.com/zpdzap/sandcastles/internal/sandbox"
)

// Run starts the main TUI loop. It cycles between the Bubble Tea dashboard
// and subprocess connections (tmux attach) until the user quits.
func Run(mgr *sandbox.Manager, cfg *config.Config) error {
	p := tea.NewProgram(newModel(mgr, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	mgr.CleanupStopped()
	fmt.Println("Goodbye! (running sandcastles left intact — use /stop to tear them down)")
	return nil
}
