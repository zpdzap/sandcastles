package tui

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zpdzap/sandcastles/internal/agent"
	"github.com/zpdzap/sandcastles/internal/sandbox"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 6 // account for "  > /" prefix
		return m, nil

	case statusTickMsg:
		m.manager.RefreshStatuses()
		// Poll tmux output and bell flag from each running sandbox
		for _, sb := range m.manager.List() {
			if sb.Status != sandbox.StatusRunning {
				continue
			}
			containerName := fmt.Sprintf("sc-%s", sb.Name)

			// Enable monitor-bell once per sandbox so tmux tracks BEL signals
			if !m.bellInit[sb.Name] {
				exec.Command("docker", "exec", containerName,
					"tmux", "set-option", "-t", "main", "monitor-bell", "on").Run()
				m.bellInit[sb.Name] = true
			}

			// Skip polling when a user is attached — our exec calls cause flicker
			clientOut, _ := exec.Command("docker", "exec", containerName,
				"tmux", "list-clients", "-t", "main").CombinedOutput()
			if strings.Contains(string(clientOut), "attached") {
				continue
			}

			// Capture pane output for preview
			out, err := exec.Command("docker", "exec", containerName,
				"tmux", "capture-pane", "-t", "main", "-p", "-S", "-30").CombinedOutput()
			if err != nil {
				continue
			}
			output := string(out)
			prevOutput := m.previews[sb.Name]
			m.previews[sb.Name] = output

			// Detect agent state: output change + bell flag
			m.agentStates[sb.Name] = detectAgentState(containerName, output, prevOutput)
		}
		// Pick up progress updates from the background create goroutine
		if m.progressPhase != nil && *m.progressPhase != "" {
			m.message = fmt.Sprintf("[%s] %s", m.progressName, *m.progressPhase)
			m.isError = false
		}
		return m, tickCmd()

	case sandboxCreatedMsg:
		m.progressName = ""
		m.progressPhase = nil
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
			m.isError = true
		} else {
			m.message = fmt.Sprintf("Created sandcastle: %s", msg.name)
			m.isError = false
		}
		return m, tea.ClearScreen

	case sandboxDestroyedMsg:
		m.message = fmt.Sprintf("Destroyed sandcastle: %s", msg.name)
		m.isError = false
		delete(m.previews, msg.name)
		delete(m.agentStates, msg.name)
		delete(m.bellInit, msg.name)
		sandboxes := m.manager.List()
		if m.cursor >= len(sandboxes) && m.cursor > 0 {
			m.cursor--
		}
		return m, tea.ClearScreen

	case allDestroyedMsg:
		m.message = fmt.Sprintf("Destroyed %d sandcastles", msg.count)
		m.isError = false
		m.cursor = 0
		m.previews = make(map[string]string)
		m.agentStates = make(map[string]string)
		m.bellInit = make(map[string]bool)
		return m, tea.ClearScreen

	case confirmStopExpiredMsg:
		m.confirmStop = false
		m.confirmStopName = ""
		return m, nil

	case tea.KeyMsg:
		if m.commanding {
			return m.handleCommandMode(msg)
		}
		return m.handleNormalMode(msg)
	}

	// Forward to input if in command mode
	if m.commanding {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleNormalMode handles keys when navigating the sandcastle list.
func (m model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dismiss help modal
	if m.showHelp {
		if msg.String() == "?" || msg.String() == "esc" {
			m.showHelp = false
			return m, nil
		}
		// While help is showing, ignore other keys
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// If confirming a stop, second x confirms, anything else cancels
	if m.confirmStop {
		m.confirmStop = false
		if msg.String() == "x" {
			name := m.confirmStopName
			m.confirmStopName = ""
			m.manager.MarkStopping(name)
			m.message = fmt.Sprintf("Stopping sandcastle %s...", name)
			m.isError = false
			return m, func() tea.Msg {
				m.manager.Destroy(name)
				return sandboxDestroyedMsg{name: name}
			}
		}
		m.confirmStopName = ""
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "/":
		m.commanding = true
		m.input.Focus()
		m.input.SetValue("")
		return m, textinput.Blink

	case "s":
		m.commanding = true
		m.input.Focus()
		m.input.SetValue("start ")
		m.input.SetCursor(6)
		return m, textinput.Blink

	case "x":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			m.confirmStop = true
			m.confirmStopName = sandboxes[m.cursor].Name
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return confirmStopExpiredMsg{}
			})
		}
		return m, nil

	case "d":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			sb := sandboxes[m.cursor]
			out, err := exec.Command("git", "-C", sb.WorktreePath, "diff").CombinedOutput()
			if err != nil {
				m.message = fmt.Sprintf("diff error: %v", err)
				m.isError = true
			} else if len(out) == 0 {
				m.message = fmt.Sprintf("[%s] No changes yet", sb.Name)
				m.isError = false
			} else {
				m.message = fmt.Sprintf("[%s diff]\n%s", sb.Name, string(out))
				m.isError = false
			}
		}
		return m, nil

	case "m":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			name := sandboxes[m.cursor].Name
			result, err := m.manager.Merge(name)
			if err != nil {
				m.message = fmt.Sprintf("Merge failed: %v", err)
				m.isError = true
			} else {
				m.message = result
				m.isError = false
			}
		}
		return m, nil

	case "r":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			name := sandboxes[m.cursor].Name
			if err := m.manager.RefreshCredentials(name); err != nil {
				m.message = fmt.Sprintf("Refresh failed: %v", err)
				m.isError = true
			} else {
				m.message = fmt.Sprintf("[%s] Credentials refreshed", name)
				m.isError = false
			}
		}
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "up", "k":
		sandboxes := m.manager.List()
		if m.cursor > 0 {
			m.cursor--
		} else if len(sandboxes) > 0 {
			m.cursor = len(sandboxes) - 1
		}
		return m, nil

	case "down", "j":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes)-1 {
			m.cursor++
		} else if len(sandboxes) > 0 {
			m.cursor = 0
		}
		return m, nil

	case "enter":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			m.connectTo = sandboxes[m.cursor].Name
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

// handleCommandMode handles keys when the command input is active.
func (m model) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		m.commanding = false
		m.input.Blur()
		m.input.SetValue("")
		return m, nil

	case "enter":
		m.commanding = false
		m.input.Blur()
		return m.processInput()
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) processInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")

	if input == "" {
		return m, nil
	}

	// Allow commands with or without the / prefix
	if input[0] == '/' {
		input = input[1:]
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	switch parts[0] {
	case "start":
		if len(parts) < 2 {
			m.message = "Usage: /start <name> [task description]"
			m.isError = true
			return m, nil
		}
		name := parts[1]
		if !validName.MatchString(name) {
			m.message = "Name must be alphanumeric (hyphens ok, e.g. my-sandbox)"
			m.isError = true
			return m, nil
		}
		task := ""
		if len(parts) > 2 {
			task = strings.Join(parts[2:], " ")
		}
		m.progressName = name
		phase := "Starting..."
		m.progressPhase = &phase
		m.message = fmt.Sprintf("[%s] Starting...", name)
		m.isError = false

		pp := m.progressPhase // capture pointer for closure
		return m, func() tea.Msg {
			progress := func(p string) {
				*pp = p
			}
			sb, err := m.manager.Create(name, task, progress)
			if err != nil {
				return sandboxCreatedMsg{name: name, err: err}
			}
			// Auto-start Claude in background (non-blocking, non-fatal)
			go agent.Start(fmt.Sprintf("sc-%s", sb.Name), task)
			return sandboxCreatedMsg{name: name}
		}

	case "stop":
		if len(parts) < 2 {
			m.message = "Usage: /stop <name> or /stop all"
			m.isError = true
			return m, nil
		}
		if parts[1] == "all" {
			sandboxes := m.manager.List()
			if len(sandboxes) == 0 {
				m.message = "No sandcastles to stop"
				m.isError = false
				return m, nil
			}
			count := len(sandboxes)
			for _, sb := range sandboxes {
				m.manager.MarkStopping(sb.Name)
			}
			m.message = fmt.Sprintf("Stopping %d sandcastles...", count)
			m.isError = false
			return m, func() tea.Msg {
				m.manager.DestroyAll()
				return allDestroyedMsg{count: count}
			}
		}
		name := parts[1]
		m.manager.MarkStopping(name)
		m.message = fmt.Sprintf("Stopping sandcastle %s...", name)
		m.isError = false
		return m, func() tea.Msg {
			m.manager.Destroy(name)
			return sandboxDestroyedMsg{name: name}
		}

	case "connect":
		if len(parts) < 2 {
			m.message = "Usage: /connect <name>"
			m.isError = true
			return m, nil
		}
		m.connectTo = parts[1]
		return m, tea.Quit

	case "diff":
		if len(parts) < 2 {
			m.message = "Usage: /diff <name>"
			m.isError = true
			return m, nil
		}
		name := parts[1]
		sb, ok := m.manager.Get(name)
		if !ok {
			m.message = fmt.Sprintf("Sandcastle %q not found", name)
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

	case "merge":
		if len(parts) < 2 {
			m.message = "Usage: /merge <name>"
			m.isError = true
			return m, nil
		}
		name := parts[1]
		result, err := m.manager.Merge(name)
		if err != nil {
			m.message = fmt.Sprintf("Merge failed: %v", err)
			m.isError = true
		} else {
			m.message = result
			m.isError = false
		}
		return m, nil

	case "refresh":
		if len(parts) < 2 {
			m.message = "Usage: /refresh <name>"
			m.isError = true
			return m, nil
		}
		name := parts[1]
		if err := m.manager.RefreshCredentials(name); err != nil {
			m.message = fmt.Sprintf("Refresh failed: %v", err)
			m.isError = true
		} else {
			m.message = fmt.Sprintf("[%s] Credentials refreshed", name)
			m.isError = false
		}
		return m, nil

	case "quit":
		m.quitting = true
		return m, tea.Quit

	default:
		m.message = fmt.Sprintf("Unknown command: %s", parts[0])
		m.isError = true
		return m, nil
	}
}

// detectAgentState infers the agent's state from output changes and UI patterns.
// Returns "working", "waiting", or "done".
//
// Detection:
//  1. Shell prompt ($) on last non-empty line → "done"
//  2. Output changed since last tick → "working" (agent producing output)
//  3. Output stable + Claude UI shows idle/prompt patterns → "waiting"
//  4. Otherwise → "working"
func detectAgentState(containerName, output, prevOutput string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Check for shell prompt — agent has exited
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			if strings.HasSuffix(trimmed, "$") || strings.HasSuffix(trimmed, "$ ") {
				return "done"
			}
			break
		}
	}

	// If output changed since last tick, agent is actively working
	if output != prevOutput && prevOutput != "" {
		return "working"
	}

	// Output is stable — check tmux bell flag (Claude sends BEL when idle)
	bellOut, err := exec.Command("docker", "exec", containerName,
		"tmux", "display-message", "-t", "main", "-p", "#{window_bell_flag}").CombinedOutput()
	if err == nil && strings.TrimSpace(string(bellOut)) == "1" {
		return "waiting"
	}

	// Scan recent lines for Claude Code UI patterns indicating idle/waiting.
	// This catches cases where the bell was missed (e.g. monitor-bell wasn't
	// enabled when the bell fired).
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-15; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// Idle indicator: ✻ (U+273B) — e.g. "✻ Churned for 2m 5s"
		if strings.HasPrefix(trimmed, "\u273b") {
			return "waiting"
		}
		// AskUserQuestion / permission prompts
		if strings.Contains(trimmed, "Enter to select") || strings.Contains(trimmed, "Esc to cancel") {
			return "waiting"
		}
	}

	return "working"
}
