package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	// Calculate column dimensions
	// Border adds 2 chars width (left+right) and 2 lines height (top+bottom)
	colWidth := m.width / len(sandboxes)
	if colWidth < 20 {
		colWidth = 20
	}
	// Inner width after border
	innerWidth := colWidth - 2

	// Available height for columns: total - header(1) - hotkeys(1) - divider(1) - status(1) - possible input(1)
	footerLines := 3
	if m.commanding {
		footerLines++
	}
	if m.message != "" {
		footerLines++
	}
	colHeight := max(5, m.height-1-footerLines)
	// Inner height after border top/bottom
	innerHeight := colHeight - 2

	// Render each column
	var columns []string
	for i, sb := range sandboxes {
		col := m.renderColumn(i, sb, innerWidth, innerHeight)

		// Pick border style based on selection
		border := columnBorder
		if i == m.cursor {
			border = columnBorderSelected
		}
		styled := border.Width(innerWidth).Height(innerHeight).Render(col)
		columns = append(columns, styled)
	}

	// Join columns horizontally
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, columns...))
	b.WriteString("\n")

	// Hotkeys
	if m.commanding {
		b.WriteString(hotkeysStyle.Render("[enter] execute  [esc] cancel"))
	} else if m.confirmStop {
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Stop %s? Press x again to confirm, any other key to cancel", m.confirmStopName)))
	} else {
		b.WriteString(hotkeysStyle.Render("[↑↓] select  [enter] connect  [s]tart  [x] stop  [d]iff  [m]erge  [r]efresh  [?] help"))
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

// renderColumn renders a single sandbox column: header line + preview content.
func (m model) renderColumn(index int, sb *sandbox.Sandbox, width, height int) string {
	// Header: icon + name (+ state label)
	icon, iStyle := m.agentIcon(sb)
	header := iStyle.Render(icon) + " " + columnHeaderStyle.Render(sb.Name)

	if sb.Status == sandbox.StatusRunning {
		switch m.agentStates[sb.Name] {
		case "waiting":
			header += " " + stateWaiting.Render("waiting")
		case "done":
			header += " " + stateDone.Render("done")
		}
	}

	// Port mappings
	portKeys := make([]string, 0, len(sb.Ports))
	for k := range sb.Ports {
		portKeys = append(portKeys, k)
	}
	sort.Strings(portKeys)
	var ports []string
	for _, container := range portKeys {
		host := sb.Ports[container]
		if container == host {
			ports = append(ports, portStyle.Render(":"+container))
		} else {
			ports = append(ports, portStyle.Render(":"+container+"→:"+host))
		}
	}
	if len(ports) > 0 {
		header += " " + strings.Join(ports, " ")
	}

	header = ansi.Truncate(header, width, "")

	// Preview content — fills remaining height below header
	contentHeight := height - 1 // 1 line for header
	if contentHeight < 1 {
		contentHeight = 1
	}

	var content string

	// Creating state
	if sb.Status == sandbox.StatusCreating || (m.progressName == sb.Name && m.progressPhase != nil) {
		phase := "Starting..."
		if m.progressPhase != nil && *m.progressPhase != "" {
			phase = *m.progressPhase
		}
		content = columnContentStyle.Render(phase)
	} else if preview, ok := m.previews[sb.Name]; ok && strings.TrimSpace(preview) != "" {
		// Show last N lines of tmux output
		lines := strings.Split(strings.TrimRight(preview, "\n"), "\n")
		if len(lines) > contentHeight {
			lines = lines[len(lines)-contentHeight:]
		}
		for i, line := range lines {
			lines[i] = ansi.Truncate(line, width, "")
		}
		content = columnContentStyle.Render(strings.Join(lines, "\n"))
	} else {
		content = columnContentStyle.Render("Waiting for output...")
	}

	return header + "\n" + content
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
		helpKeyStyle.Render("  r") + helpDescStyle.Render("           Refresh credentials"),
		"",
		helpHeaderStyle.Render("Commands"),
		helpKeyStyle.Render("  /") + helpDescStyle.Render("           Open command bar"),
		helpDescStyle.Render("  /start <name> [task]"),
		helpDescStyle.Render("  /stop <name|all>"),
		helpDescStyle.Render("  /connect <name>"),
		helpDescStyle.Render("  /diff <name>"),
		helpDescStyle.Render("  /merge <name>"),
		helpDescStyle.Render("  /refresh <name>"),
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
