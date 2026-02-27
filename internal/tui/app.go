package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zpdzap/sandcastles/internal/config"
	"github.com/zpdzap/sandcastles/internal/sandbox"
)

// Run starts the main TUI loop. It cycles between the Bubble Tea dashboard
// and subprocess connections (tmux attach) until the user quits.
func Run(mgr *sandbox.Manager, cfg *config.Config) error {
	for {
		m := newModel(mgr, cfg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		result, err := p.Run()
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		final := result.(model)

		if final.quitting {
			fmt.Println("Shutting down sandcastles...")
			mgr.DestroyAll()
			fmt.Println("Goodbye!")
			return nil
		}

		if final.connectTo != "" {
			fmt.Printf("Connecting to %s... (detach tmux with Ctrl-B d to return)\n", final.connectTo)

			cmd := mgr.ConnectCmd(final.connectTo)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()

			fmt.Println("Returning to dashboard...")
		}
	}
}
