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

// setMessage sets the status bar message and returns a tea.Cmd to auto-clear it.
// Errors clear after 10s, normal messages after 5s.
func (m *model) setMessage(text string, isErr bool) tea.Cmd {
	m.messageID++
	m.message = text
	m.isError = isErr
	d := 5 * time.Second
	if isErr {
		d = 10 * time.Second
	}
	id := m.messageID
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearMessageMsg{id: id}
	})
}

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

			// Skip ALL docker exec calls when a client was recently attached.
			// Even list-clients causes flicker during rapid output, so once we
			// detect attachment we back off entirely for 10 seconds.
			if t, ok := m.attachedAt[sb.Name]; ok && time.Since(t) < 4*time.Second {
				continue
			}

			// Check if a user is attached — if so, cache and skip
			clientOut, _ := exec.Command("docker", "exec", containerName,
				"tmux", "list-clients", "-t", "main").CombinedOutput()
			if strings.Contains(string(clientOut), "attached") {
				m.attachedAt[sb.Name] = time.Now()
				continue
			}
			delete(m.attachedAt, sb.Name)

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

			// Fetch lightweight diff stats for column header
			m.diffStats[sb.Name] = fetchDiffStats(sb.Name)
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
		var clearCmd tea.Cmd
		if msg.err != nil {
			clearCmd = m.setMessage(fmt.Sprintf("Error: %v", msg.err), true)
		} else {
			clearCmd = m.setMessage(fmt.Sprintf("Created sandcastle: %s", msg.name), false)
		}
		return m, tea.Batch(tea.ClearScreen, clearCmd)

	case sandboxDestroyedMsg:
		clearCmd := m.setMessage(fmt.Sprintf("Destroyed sandcastle: %s", msg.name), false)
		delete(m.previews, msg.name)
		delete(m.agentStates, msg.name)
		delete(m.diffStats, msg.name)
		delete(m.bellInit, msg.name)
		delete(m.attachedAt, msg.name)
		sandboxes := m.manager.List()
		if m.cursor >= len(sandboxes) && m.cursor > 0 {
			m.cursor--
		}
		return m, tea.Batch(tea.ClearScreen, clearCmd)

	case allDestroyedMsg:
		clearCmd := m.setMessage(fmt.Sprintf("Destroyed %d sandcastles", msg.count), false)
		m.cursor = 0
		m.previews = make(map[string]string)
		m.agentStates = make(map[string]string)
		m.diffStats = make(map[string]diffStat)
		m.bellInit = make(map[string]bool)
		m.attachedAt = make(map[string]time.Time)
		return m, tea.Batch(tea.ClearScreen, clearCmd)

	case attachFinishedMsg:
		// User detached from tmux — suppress polling briefly to avoid flicker
		m.attachedAt[msg.name] = time.Now()
		return m, tea.ClearScreen

	case clearMessageMsg:
		if msg.id == m.messageID {
			m.message = ""
			m.isError = false
		}
		return m, nil

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
	// Dismiss modals
	if m.showHelp {
		if msg.String() == "?" || msg.String() == "esc" {
			m.showHelp = false
			return m, nil
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}
	if m.showDiff {
		if msg.String() == "d" || msg.String() == "esc" {
			m.showDiff = false
			m.diffContent = ""
			return m, nil
		}
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
			tree, err := buildDiffTree(sb.WorktreePath, sb.Name)
			if err != nil {
				return m, m.setMessage(fmt.Sprintf("diff error: %v", err), true)
			}
			m.diffContent = tree
			m.showDiff = true
		}
		return m, nil

	case "m":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			name := sandboxes[m.cursor].Name
			result, err := m.manager.Merge(name)
			if err != nil {
				return m, m.setMessage(fmt.Sprintf("Merge failed: %v", err), true)
			}
			return m, m.setMessage(result, false)
		}
		return m, nil

	case "r":
		sandboxes := m.manager.List()
		if m.cursor < len(sandboxes) {
			name := sandboxes[m.cursor].Name
			if err := m.manager.RefreshCredentials(name); err != nil {
				return m, m.setMessage(fmt.Sprintf("Reauth failed: %v", err), true)
			}
			return m, m.setMessage(fmt.Sprintf("[%s] Credentials reauthorized", name), false)
		}
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "left", "h":
		sandboxes := m.manager.List()
		if m.cursor > 0 {
			m.cursor--
		} else if len(sandboxes) > 0 {
			m.cursor = len(sandboxes) - 1
		}
		return m, nil

	case "right", "l":
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
			name := sandboxes[m.cursor].Name
			m.attachedAt[name] = time.Now()
			cmd := m.manager.ConnectCmd(name)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				return attachFinishedMsg{name: name}
			})
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
			return m, m.setMessage("Usage: /start <name> [task description]", true)
		}
		name := parts[1]
		if !validName.MatchString(name) {
			return m, m.setMessage("Name must be alphanumeric (hyphens ok, e.g. my-sandbox)", true)
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
			return m, m.setMessage("Usage: /stop <name> or /stop all", true)
		}
		if parts[1] == "all" {
			sandboxes := m.manager.List()
			if len(sandboxes) == 0 {
				return m, m.setMessage("No sandcastles to stop", false)
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
			return m, m.setMessage("Usage: /connect <name>", true)
		}
		name := parts[1]
		m.attachedAt[name] = time.Now()
		cmd := m.manager.ConnectCmd(name)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return attachFinishedMsg{name: name}
		})

	case "diff":
		if len(parts) < 2 {
			return m, m.setMessage("Usage: /diff <name>", true)
		}
		name := parts[1]
		sb, ok := m.manager.Get(name)
		if !ok {
			return m, m.setMessage(fmt.Sprintf("Sandcastle %q not found", name), true)
		}
		tree, err := buildDiffTree(sb.WorktreePath, name)
		if err != nil {
			return m, m.setMessage(fmt.Sprintf("diff error: %v", err), true)
		}
		m.diffContent = tree
		m.showDiff = true
		return m, nil

	case "merge":
		if len(parts) < 2 {
			return m, m.setMessage("Usage: /merge <name>", true)
		}
		name := parts[1]
		result, err := m.manager.Merge(name)
		if err != nil {
			return m, m.setMessage(fmt.Sprintf("Merge failed: %v", err), true)
		}
		return m, m.setMessage(result, false)

	case "reauth":
		if len(parts) < 2 {
			return m, m.setMessage("Usage: /reauth <name>", true)
		}
		name := parts[1]
		if err := m.manager.RefreshCredentials(name); err != nil {
			return m, m.setMessage(fmt.Sprintf("Reauth failed: %v", err), true)
		}
		return m, m.setMessage(fmt.Sprintf("[%s] Credentials reauthorized", name), false)

	case "quit":
		m.quitting = true
		return m, tea.Quit

	default:
		return m, m.setMessage(fmt.Sprintf("Unknown command: %s", parts[0]), true)
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
