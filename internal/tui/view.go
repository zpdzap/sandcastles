package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zpdzap/sandcastles/internal/sandbox"
)

func (m model) View() string {
	if m.quitting {
		return ""
	}

	sandboxes := m.manager.List()

	// Header — always shown
	title := "sandcastles v0.1.0"
	quip := quipStyle.Render(m.quip)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(quip) - 4
	if gap < 1 {
		gap = 1
	}
	header := headerStyle.Width(m.width).Render(title + strings.Repeat(" ", gap) + quip)

	// Empty state — same as before but with updated hotkey hint
	if len(sandboxes) == 0 {
		return m.renderEmptyState(header)
	}

	// Split view
	return m.renderSplitView(header, sandboxes)
}

func (m model) renderEmptyState(header string) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")
	b.WriteString(emptyStyle.Render("No sandcastles running. Press s or / to start one."))
	b.WriteString("\n\n")

	if m.commanding {
		b.WriteString(hotkeysStyle.Render("[enter] execute  [esc] cancel"))
	} else {
		b.WriteString(hotkeysStyle.Render("[s]tart  [?] help  [q] quit"))
	}
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Status message
	m.renderStatusAndInput(&b)

	// Help overlay
	if m.showHelp {
		return m.renderHelpOverlay(b.String())
	}

	return b.String()
}

func (m model) renderSplitView(header string, sandboxes []*sandbox.Sandbox) string {
	var b strings.Builder

	// Header
	b.WriteString(header)
	b.WriteString("\n")

	// Sandbox list — one line per sandbox
	for i, sb := range sandboxes {
		b.WriteString(m.renderSandbox(i, sb))
		b.WriteString("\n")
	}

	// Divider
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Preview pane — fill remaining vertical space
	// Calculate available height: total - header(1) - sandboxes(N) - divider(1) - preview divider(1) - hotkeys(1) - divider(1) - status(1) - input(1 if commanding)
	footerLines := 4 // hotkeys + divider + status + possible input
	if m.commanding {
		footerLines++
	}
	previewHeight := max(3, m.height-1-len(sandboxes)-1-1-footerLines)

	b.WriteString(m.renderPreview(sandboxes, previewHeight))

	// Bottom divider
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Hotkeys
	if m.commanding {
		b.WriteString(hotkeysStyle.Render("[enter] execute  [esc] cancel"))
	} else if m.confirmStop {
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Stop %s? Press x again to confirm, any other key to cancel", m.confirmStopName)))
	} else {
		b.WriteString(hotkeysStyle.Render("[↑↓] select  [enter] connect  [s]tart  [x] stop  [d]iff  [m]erge  [?] help"))
	}
	b.WriteString("\n")

	// Status message and input
	m.renderStatusAndInput(&b)

	// Help overlay
	if m.showHelp {
		return m.renderHelpOverlay(b.String())
	}

	return b.String()
}

func (m model) renderPreview(sandboxes []*sandbox.Sandbox, height int) string {
	var b strings.Builder

	// Get the selected sandbox
	if m.cursor >= len(sandboxes) {
		b.WriteString(previewEmptyStyle.Render("No sandbox selected"))
		b.WriteString("\n")
		for i := 1; i < height; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	selected := sandboxes[m.cursor]

	// If creating, show progress phase
	if selected.Status == sandbox.StatusCreating || (m.progressName == selected.Name && m.progressPhase != nil) {
		phase := "Starting..."
		if m.progressPhase != nil && *m.progressPhase != "" {
			phase = *m.progressPhase
		}
		b.WriteString(previewEmptyStyle.Render(fmt.Sprintf("[%s] %s", selected.Name, phase)))
		b.WriteString("\n")
		for i := 1; i < height; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	// Show tmux preview
	preview, ok := m.previews[selected.Name]
	if !ok || strings.TrimSpace(preview) == "" {
		b.WriteString(previewEmptyStyle.Render("Waiting for output..."))
		b.WriteString("\n")
		for i := 1; i < height; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	// Take last N lines to fit the preview height
	lines := strings.Split(strings.TrimRight(preview, "\n"), "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}

	for _, line := range lines {
		// Truncate lines to terminal width
		if lipgloss.Width(line) > m.width-4 {
			line = line[:m.width-4]
		}
		b.WriteString(previewStyle.Render(line))
		b.WriteString("\n")
	}

	// Pad remaining lines
	for i := len(lines); i < height; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderSandbox(index int, sb *sandbox.Sandbox) string {
	cursor := "  "
	nStyle := nameStyle
	if index == m.cursor {
		cursor = "▸ "
		nStyle = selectedNameStyle
	}

	// Agent state icon (overrides container status for running containers)
	icon, iStyle := m.agentIcon(sb)
	status := iStyle.Render(icon)
	name := nStyle.Render(sb.Name)

	var parts []string
	parts = append(parts, fmt.Sprintf("  %s%s %s", cursor, status, name))

	// Agent state label for running containers
	if sb.Status == sandbox.StatusRunning {
		state := m.agentStates[sb.Name]
		switch state {
		case "waiting":
			parts = append(parts, stateWaiting.Render("waiting"))
		case "done":
			parts = append(parts, stateDone.Render("done"))
		default:
			// Don't show "working" label — the green icon is enough
		}
	}

	// Show port mappings (sorted for stable display)
	portKeys := make([]string, 0, len(sb.Ports))
	for k := range sb.Ports {
		portKeys = append(portKeys, k)
	}
	sort.Strings(portKeys)
	for _, container := range portKeys {
		host := sb.Ports[container]
		if container == host {
			parts = append(parts, portStyle.Render(fmt.Sprintf(":%s", container)))
		} else {
			parts = append(parts, portStyle.Render(fmt.Sprintf(":%s→:%s", container, host)))
		}
	}

	return strings.Join(parts, "  ")
}

// agentIcon returns the status icon and style for a sandbox.
// For running sandboxes, it reflects agent state; otherwise container status.
func (m model) agentIcon(sb *sandbox.Sandbox) (string, lipgloss.Style) {
	if sb.Status == sandbox.StatusRunning {
		state := m.agentStates[sb.Name]
		switch state {
		case "waiting":
			return "◎", stateWaiting
		case "done":
			return "✓", stateDone
		default:
			return "●", stateWorking
		}
	}
	switch sb.Status {
	case sandbox.StatusStopping:
		return "◍", statusOther
	case sandbox.StatusStopped:
		return "○", statusStopped
	default:
		return "◌", statusOther
	}
}

func (m model) renderStatusAndInput(b *strings.Builder) {
	if m.message != "" {
		if m.isError {
			b.WriteString(errorStyle.Render(m.message))
		} else {
			b.WriteString(messageStyle.Render(m.message))
		}
		b.WriteString("\n")
	}
	if m.commanding {
		b.WriteString("  ")
		b.WriteString(m.input.View())
		b.WriteString("\n")
	}
}

func (m model) renderHelpOverlay(base string) string {
	help := strings.Join([]string{
		helpHeaderStyle.Render("Navigation"),
		helpKeyStyle.Render("  ↑/k  ↓/j") + helpDescStyle.Render("   Select sandbox"),
		helpKeyStyle.Render("  Enter") + helpDescStyle.Render("       Connect (tmux attach)"),
		"",
		helpHeaderStyle.Render("Actions"),
		helpKeyStyle.Render("  s") + helpDescStyle.Render("           Start a new sandbox"),
		helpKeyStyle.Render("  x") + helpDescStyle.Render("           Stop selected sandbox"),
		helpKeyStyle.Render("  d") + helpDescStyle.Render("           Diff selected sandbox"),
		helpKeyStyle.Render("  m") + helpDescStyle.Render("           Merge selected sandbox"),
		"",
		helpHeaderStyle.Render("Commands"),
		helpKeyStyle.Render("  /") + helpDescStyle.Render("           Open command bar"),
		helpDescStyle.Render("  /start <name> [task]"),
		helpDescStyle.Render("  /stop <name|all>"),
		helpDescStyle.Render("  /connect <name>"),
		helpDescStyle.Render("  /diff <name>"),
		helpDescStyle.Render("  /merge <name>"),
		"",
		helpKeyStyle.Render("  q") + helpDescStyle.Render("  quit") + "     " + helpKeyStyle.Render("?") + helpDescStyle.Render("  close this help"),
	}, "\n")

	modal := helpStyle.Render(help)

	// Center the modal over the base view
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	baseLines := strings.Split(base, "\n")

	// Calculate offsets
	xOffset := max(0, (m.width-modalWidth)/2)
	yOffset := max(0, (m.height-modalHeight)/2)

	// Overlay modal onto base
	modalLines := strings.Split(modal, "\n")
	for i, mLine := range modalLines {
		row := yOffset + i
		if row < len(baseLines) {
			baseLine := baseLines[row]
			// Replace the portion of the base line with the modal line
			padding := strings.Repeat(" ", xOffset)
			// Ensure base line is wide enough
			for lipgloss.Width(baseLine) < m.width {
				baseLine += " "
			}
			baseLines[row] = padding + mLine + strings.Repeat(" ", max(0, m.width-xOffset-lipgloss.Width(mLine)))
		}
	}

	return strings.Join(baseLines, "\n")
}
