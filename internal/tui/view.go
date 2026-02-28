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

	var b strings.Builder

	// Header
	header := headerStyle.Width(m.width).Render("sandcastles v0.1.0")
	sandboxes := m.manager.List()

	noun := "sandcastles"
	if len(sandboxes) == 1 {
		noun = "sandcastle"
	}
	stats := statsStyle.Width(m.width).Render(fmt.Sprintf("%d %s", len(sandboxes), noun))

	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(stats)
	b.WriteString("\n")

	// Divider
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Sandbox list
	if len(sandboxes) == 0 {
		b.WriteString(emptyStyle.Render("No sandcastles running. Press / then type: start <name> [task]"))
		b.WriteString("\n")
	} else {
		for i, sb := range sandboxes {
			b.WriteString(m.renderSandbox(i, sb))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Hotkeys — different depending on mode
	if m.commanding {
		b.WriteString(hotkeysStyle.Render("[enter] execute  [esc] cancel"))
	} else {
		b.WriteString(hotkeysStyle.Render("[↑↓] select  [enter] connect  [/] command  [q] quit"))
	}
	b.WriteString("\n")

	// Divider
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Status message
	if m.message != "" {
		if m.isError {
			b.WriteString(errorStyle.Render(m.message))
		} else {
			b.WriteString(messageStyle.Render(m.message))
		}
		b.WriteString("\n")
	}

	// Command input — only visible in command mode
	if m.commanding {
		b.WriteString("  / ")
		b.WriteString(m.input.View())
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

	statusIcon, sStyle := statusIndicator(sb.Status)
	status := sStyle.Render(statusIcon)
	name := nStyle.Render(sb.Name)

	var parts []string
	parts = append(parts, fmt.Sprintf("  %s%s %s", cursor, status, name))

	if sb.Task != "" {
		parts = append(parts, taskStyle.Render(sb.Task))
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
			// Host networking — no mapping needed
			parts = append(parts, portStyle.Render(fmt.Sprintf(":%s", container)))
		} else {
			parts = append(parts, portStyle.Render(fmt.Sprintf(":%s→:%s", container, host)))
		}
	}

	return strings.Join(parts, "  ")
}

func statusIndicator(s sandbox.Status) (string, lipgloss.Style) {
	switch s {
	case sandbox.StatusRunning:
		return "●", statusRunning
	case sandbox.StatusStopping:
		return "◍", statusOther
	case sandbox.StatusStopped:
		return "○", statusStopped
	default:
		return "◌", statusOther
	}
}
