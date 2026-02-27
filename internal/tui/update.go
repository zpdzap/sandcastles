package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zpdzap/sandcastles/internal/agent"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		return m, nil

	case statusTickMsg:
		m.manager.RefreshStatuses()
		return m, tickCmd()

	case sandboxProgressMsg:
		m.message = fmt.Sprintf("[%s] %s", msg.name, msg.phase)
		m.isError = false
		return m, listenForProgress(m.progressCh)

	case sandboxCreatedMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
			m.isError = true
		} else {
			m.message = fmt.Sprintf("Created sandbox: %s", msg.name)
			m.isError = false
		}
		return m, nil

	case sandboxDestroyedMsg:
		m.message = fmt.Sprintf("Destroyed sandbox: %s", msg.name)
		m.isError = false
		sandboxes := m.manager.List()
		if m.cursor >= len(sandboxes) && m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up":
		sandboxes := m.manager.List()
		if m.cursor > 0 {
			m.cursor--
		} else if len(sandboxes) > 0 {
			m.cursor = len(sandboxes) - 1
		}
		return m, nil

	case "down":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes)-1 {
			m.cursor++
		}
		return m, nil

	case "enter":
		if m.input.Value() != "" {
			return m.processInput()
		}
		// Connect to selected sandbox
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			m.connectTo = sandboxes[m.cursor].Name
			return m, tea.Quit
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) processInput() (tea.Model, tea.Cmd) {
	input := m.input.Value()
	m.input.SetValue("")

	cmd := ParseCommand(input)
	if cmd == nil {
		m.message = "Commands start with /. Try /start, /stop, /connect, /diff, /quit"
		m.isError = true
		return m, nil
	}

	switch cmd.Name {
	case "/start":
		if len(cmd.Args) < 1 {
			m.message = "Usage: /start <name> [task description]"
			m.isError = true
			return m, nil
		}
		name := cmd.Args[0]
		task := ""
		if len(cmd.Args) > 1 {
			task = strings.Join(cmd.Args[1:], " ")
		}
		m.message = fmt.Sprintf("[%s] Starting...", name)
		m.isError = false

		// Create a progress channel for streaming build phases
		ch := make(chan sandboxProgressMsg, 8)
		m.progressCh = ch

		createCmd := func() tea.Msg {
			progress := func(phase string) {
				ch <- sandboxProgressMsg{name: name, phase: phase}
			}
			sb, err := m.manager.Create(name, task, progress)
			close(ch)
			if err != nil {
				return sandboxCreatedMsg{name: name, err: err}
			}
			// Try to auto-start agent (non-fatal)
			if task != "" {
				containerName := fmt.Sprintf("sc-%s", sb.Name)
				if err := agent.Start(containerName, task); err != nil {
					// swallow — user will see in the tmux session
				}
			}
			return sandboxCreatedMsg{name: name}
		}

		return m, tea.Batch(createCmd, listenForProgress(ch))

	case "/stop":
		if len(cmd.Args) < 1 {
			m.message = "Usage: /stop <name>"
			m.isError = true
			return m, nil
		}
		name := cmd.Args[0]
		m.manager.MarkStopping(name)
		m.message = fmt.Sprintf("Stopping sandbox %s...", name)
		m.isError = false
		return m, func() tea.Msg {
			m.manager.Destroy(name)
			return sandboxDestroyedMsg{name: name}
		}

	case "/connect":
		if len(cmd.Args) < 1 {
			m.message = "Usage: /connect <name>"
			m.isError = true
			return m, nil
		}
		m.connectTo = cmd.Args[0]
		return m, tea.Quit

	case "/diff":
		if len(cmd.Args) < 1 {
			m.message = "Usage: /diff <name>"
			m.isError = true
			return m, nil
		}
		name := cmd.Args[0]
		sb, ok := m.manager.Get(name)
		if !ok {
			m.message = fmt.Sprintf("Sandbox %q not found", name)
			m.isError = true
			return m, nil
		}
		out, err := exec.Command("git", "-C", sb.WorktreePath, "diff").CombinedOutput()
		if err != nil {
			m.message = fmt.Sprintf("diff error: %v", err)
			m.isError = true
		} else if len(out) == 0 {
			m.message = fmt.Sprintf("[%s] No changes yet", name)
			m.isError = false
		} else {
			m.message = fmt.Sprintf("[%s diff]\n%s", name, string(out))
			m.isError = false
		}
		return m, nil

	case "/quit":
		m.quitting = true
		return m, tea.Quit

	default:
		m.message = fmt.Sprintf("Unknown command: %s", cmd.Name)
		m.isError = true
		return m, nil
	}
}
