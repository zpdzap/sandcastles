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
	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC00"))
	diffDelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	diffFileStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	diffDirStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#5599FF")).Bold(true)
	diffNewStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC00")).Bold(true)
	diffDelFileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	diffTreeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

type diffEntry struct {
	path   string
	status string // "M", "A", "D", "R"
	added  int
	deleted int
}

type dirNode struct {
	name     string
	children map[string]*dirNode
	files    []diffEntry
}

func newDirNode(name string) *dirNode {
	return &dirNode{name: name, children: make(map[string]*dirNode)}
}

// buildDiffTree runs git diff and returns a rendered file tree string.
func buildDiffTree(worktreePath, sandboxName string) (string, error) {
	// Find the merge base with main to show ALL changes on this branch
	// (committed + uncommitted) rather than just unstaged edits.
	mergeBase, err := exec.Command("git", "-C", worktreePath, "merge-base", "main", "HEAD").CombinedOutput()
	if err != nil {
		// Fallback to just HEAD if merge-base fails
		mergeBase = []byte("HEAD")
	}
	base := strings.TrimSpace(string(mergeBase))

	// Compare merge-base to working tree (includes committed + uncommitted changes)
	statusOut, err := exec.Command("git", "-C", worktreePath, "diff", "--name-status", base).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff --name-status: %w", err)
	}
	if len(strings.TrimSpace(string(statusOut))) == 0 {
		return fmt.Sprintf("[%s] No changes yet", sandboxName), nil
	}

	// Get line counts against same base
	numstatOut, _ := exec.Command("git", "-C", worktreePath, "diff", "--numstat", base).CombinedOutput()

	// Also check for untracked (new) files via status
	untrackedOut, _ := exec.Command("git", "-C", worktreePath, "status", "--porcelain").CombinedOutput()

	// Parse entries
	entries := parseGitDiff(string(statusOut), string(numstatOut), string(untrackedOut))

	// Build tree
	root := newDirNode("")
	for _, e := range entries {
		parts := strings.Split(e.path, "/")
		node := root
		for _, dir := range parts[:len(parts)-1] {
			if _, ok := node.children[dir]; !ok {
				node.children[dir] = newDirNode(dir)
			}
			node = node.children[dir]
		}
		node.files = append(node.files, e)
	}

	// Render
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(sandboxName))
	b.WriteString("\n")
	renderTree(&b, root, "")

	// Summary
	totalAdd, totalDel, fileCount := 0, 0, 0
	for _, e := range entries {
		totalAdd += e.added
		totalDel += e.deleted
		fileCount++
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

	return b.String(), nil
}

func parseGitDiff(statusStr, numstatStr, untrackedStr string) []diffEntry {
	// Parse --name-status
	statusMap := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(statusStr), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			status := parts[0][:1] // Take first char (R100 -> R)
			file := parts[len(parts)-1] // For renames, take the new name
			statusMap[file] = status
		}
	}

	// Parse --numstat
	numMap := make(map[string][2]int) // [added, deleted]
	for _, line := range strings.Split(strings.TrimSpace(numstatStr), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			added, _ := strconv.Atoi(parts[0])
			deleted, _ := strconv.Atoi(parts[1])
			file := parts[2]
			numMap[file] = [2]int{added, deleted}
		}
	}

	// Parse untracked files from porcelain status
	for _, line := range strings.Split(strings.TrimSpace(untrackedStr), "\n") {
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		file := strings.TrimSpace(line[2:])
		if code == "??" {
			if _, exists := statusMap[file]; !exists {
				statusMap[file] = "A"
			}
		}
	}

	// Merge into entries
	var entries []diffEntry
	for file, status := range statusMap {
		nums := numMap[file]
		entries = append(entries, diffEntry{
			path:    file,
			status:  status,
			added:   nums[0],
			deleted: nums[1],
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})
	return entries
}

func renderTree(b *strings.Builder, node *dirNode, prefix string) {
	// Collect and sort children
	var dirNames []string
	for name := range node.children {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)

	// Total items at this level
	items := make([]struct {
		isDir bool
		name  string
	}, 0, len(dirNames)+len(node.files))
	for _, d := range dirNames {
		items = append(items, struct {
			isDir bool
			name  string
		}{true, d})
	}
	for _, f := range node.files {
		parts := strings.Split(f.path, "/")
		items = append(items, struct {
			isDir bool
			name  string
		}{false, parts[len(parts)-1]})
	}

	for i, item := range items {
		isLast := i == len(items)-1
		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		if item.isDir {
			b.WriteString(diffTreeStyle.Render(prefix+connector) + diffDirStyle.Render(item.name+"/") + "\n")
			renderTree(b, node.children[item.name], prefix+childPrefix)
		} else {
			// Find the entry for this file
			var entry diffEntry
			for _, f := range node.files {
				parts := strings.Split(f.path, "/")
				if parts[len(parts)-1] == item.name {
					entry = f
					break
				}
			}

			// Status indicator
			var statusLabel string
			nameStyle := diffFileStyle
			switch entry.status {
			case "A":
				statusLabel = diffNewStyle.Render(" [new]")
				nameStyle = diffNewStyle
			case "D":
				statusLabel = diffDelFileStyle.Render(" [deleted]")
				nameStyle = diffDelFileStyle
			}

			// Line counts
			var counts string
			if entry.added > 0 || entry.deleted > 0 {
				parts := []string{}
				if entry.added > 0 {
					parts = append(parts, diffAddStyle.Render(fmt.Sprintf("+%d", entry.added)))
				}
				if entry.deleted > 0 {
					parts = append(parts, diffDelStyle.Render(fmt.Sprintf("-%d", entry.deleted)))
				}
				counts = " " + strings.Join(parts, " ")
			}

			b.WriteString(diffTreeStyle.Render(prefix+connector) + nameStyle.Render(item.name) + statusLabel + counts + "\n")
		}
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
