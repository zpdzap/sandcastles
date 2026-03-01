package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	diffAddStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC00"))
	diffDelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	diffFileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	diffDirStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5599FF")).Bold(true)
	diffNewStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC00")).Bold(true)
	diffDelFileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	diffTreeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	diffWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	diffHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700"))
	diffDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
)

type diffEntry struct {
	path       string
	status     string // "M", "A", "D", "R"
	added      int
	deleted    int
	uncommitted bool // true if file has uncommitted changes (unstaged, untracked)
}

type dirNode struct {
	name     string
	children map[string]*dirNode
	files    []diffEntry
}

func newDirNode(name string) *dirNode {
	return &dirNode{name: name, children: make(map[string]*dirNode)}
}

// buildDiffTree runs git commands and returns a rendered file tree string.
func buildDiffTree(worktreePath, sandboxName string) (string, error) {
	// 1. Count commits on this branch
	commitOut, _ := exec.Command("git", "-C", worktreePath,
		"rev-list", "--count", "main..HEAD").CombinedOutput()
	commitCount := strings.TrimSpace(string(commitOut))
	if commitCount == "" {
		commitCount = "0"
	}

	// 2. Committed changes: diff main..HEAD (what merge will apply)
	committedStatus, err := exec.Command("git", "-C", worktreePath,
		"diff", "--name-status", "main..HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff main..HEAD: %w", err)
	}
	committedNumstat, _ := exec.Command("git", "-C", worktreePath,
		"diff", "--numstat", "main..HEAD").CombinedOutput()

	// 3. Working tree status for uncommitted changes
	porcelainOut, _ := exec.Command("git", "-C", worktreePath,
		"status", "--porcelain").CombinedOutput()

	// Parse committed changes
	entries := parseCommitted(string(committedStatus), string(committedNumstat))

	// Overlay uncommitted changes
	overlayUncommitted(entries, string(porcelainOut))

	if len(entries) == 0 {
		return fmt.Sprintf("[%s] No changes yet", sandboxName), nil
	}

	// Sort
	sortedFiles := make([]string, 0, len(entries))
	for f := range entries {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Strings(sortedFiles)

	// Build tree
	root := newDirNode("")
	for _, f := range sortedFiles {
		e := entries[f]
		parts := strings.Split(e.path, "/")
		node := root
		for _, dir := range parts[:len(parts)-1] {
			if _, ok := node.children[dir]; !ok {
				node.children[dir] = newDirNode(dir)
			}
			node = node.children[dir]
		}
		node.files = append(node.files, *e)
	}

	// Render header
	var b strings.Builder
	b.WriteString(diffHeaderStyle.Render(sandboxName))
	n, _ := strconv.Atoi(commitCount)
	b.WriteString(diffDimStyle.Render(fmt.Sprintf("  %d commit%s", n, plural(n))))
	b.WriteString("\n")

	// Render tree
	renderTree(&b, root, "")

	// Summary line
	totalAdd, totalDel, fileCount, uncommittedCount := 0, 0, 0, 0
	for _, e := range entries {
		totalAdd += e.added
		totalDel += e.deleted
		fileCount++
		if e.uncommitted {
			uncommittedCount++
		}
	}
	b.WriteString("\n")
	summary := fmt.Sprintf("%d file%s changed", fileCount, plural(fileCount))
	if totalAdd > 0 {
		summary += ", " + diffAddStyle.Render(fmt.Sprintf("+%d", totalAdd))
	}
	if totalDel > 0 {
		summary += ", " + diffDelStyle.Render(fmt.Sprintf("-%d", totalDel))
	}
	b.WriteString(summary)

	if uncommittedCount > 0 {
		b.WriteString("\n")
		b.WriteString(diffWarnStyle.Render(fmt.Sprintf("⚠ %d file%s with uncommitted changes",
			uncommittedCount, plural(uncommittedCount))))
	}

	return b.String(), nil
}

// parseCommitted parses git diff --name-status and --numstat output.
func parseCommitted(statusStr, numstatStr string) map[string]*diffEntry {
	entries := make(map[string]*diffEntry)

	for _, line := range strings.Split(strings.TrimSpace(statusStr), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			status := parts[0][:1]
			file := parts[len(parts)-1]
			entries[file] = &diffEntry{path: file, status: status}
		}
	}

	for _, line := range strings.Split(strings.TrimSpace(numstatStr), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			added, _ := strconv.Atoi(parts[0])
			deleted, _ := strconv.Atoi(parts[1])
			file := parts[2]
			if e, ok := entries[file]; ok {
				e.added = added
				e.deleted = deleted
			}
		}
	}

	return entries
}

// overlayUncommitted adds uncommitted working tree changes on top of committed entries.
func overlayUncommitted(entries map[string]*diffEntry, porcelainStr string) {
	for _, line := range strings.Split(strings.TrimSpace(porcelainStr), "\n") {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		file := strings.TrimSpace(line[3:])

		if x == '?' && y == '?' {
			// Untracked file — not committed, needs attention
			if _, exists := entries[file]; !exists {
				entries[file] = &diffEntry{
					path:        file,
					status:      "A",
					uncommitted: true,
				}
			}
			continue
		}

		if e, exists := entries[file]; exists {
			// File is in committed diff AND has working tree changes
			if y == 'M' || y == 'D' || x == 'M' || x == 'A' || x == 'D' {
				e.uncommitted = true
			}
		} else {
			// File NOT in committed diff but has working tree changes
			var status string
			switch {
			case y == 'D' || x == 'D':
				status = "D"
			case y == 'M' || x == 'M':
				status = "M"
			case x == 'A':
				status = "A"
			default:
				continue
			}
			entries[file] = &diffEntry{
				path:        file,
				status:      status,
				uncommitted: true,
			}
		}
	}
}

func renderTree(b *strings.Builder, node *dirNode, prefix string) {
	// Collect and sort children
	var dirNames []string
	for name := range node.children {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)

	// Build ordered items at this level: dirs first, then files
	type item struct {
		isDir bool
		name  string
	}
	items := make([]item, 0, len(dirNames)+len(node.files))
	for _, d := range dirNames {
		items = append(items, item{true, d})
	}
	for _, f := range node.files {
		parts := strings.Split(f.path, "/")
		items = append(items, item{false, parts[len(parts)-1]})
	}

	for i, it := range items {
		isLast := i == len(items)-1
		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		if it.isDir {
			b.WriteString(diffTreeStyle.Render(prefix+connector) + diffDirStyle.Render(it.name+"/") + "\n")
			renderTree(b, node.children[it.name], prefix+childPrefix)
		} else {
			// Find the entry
			var entry diffEntry
			for _, f := range node.files {
				parts := strings.Split(f.path, "/")
				if parts[len(parts)-1] == it.name {
					entry = f
					break
				}
			}
			renderFileEntry(b, prefix+connector, entry)
		}
	}
}

func renderFileEntry(b *strings.Builder, prefix string, entry diffEntry) {
	nameStyle := diffFileStyle
	var badges []string

	// File status badge
	switch entry.status {
	case "A":
		nameStyle = diffNewStyle
		badges = append(badges, diffNewStyle.Render("new"))
	case "D":
		nameStyle = diffDelFileStyle
		badges = append(badges, diffDelFileStyle.Render("deleted"))
	}

	// Uncommitted warning
	if entry.uncommitted {
		badges = append(badges, diffWarnStyle.Render("uncommitted"))
	}

	// Line counts
	var counts []string
	if entry.added > 0 {
		counts = append(counts, diffAddStyle.Render(fmt.Sprintf("+%d", entry.added)))
	}
	if entry.deleted > 0 {
		counts = append(counts, diffDelStyle.Render(fmt.Sprintf("-%d", entry.deleted)))
	}

	// Build the line
	parts := strings.Split(entry.path, "/")
	fileName := parts[len(parts)-1]

	line := diffTreeStyle.Render(prefix) + nameStyle.Render(fileName)
	if len(badges) > 0 {
		line += " " + strings.Join(badges, " ")
	}
	if len(counts) > 0 {
		line += "  " + strings.Join(counts, " ")
	}
	b.WriteString(line + "\n")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
