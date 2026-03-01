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
	path        string
	status      string // "M", "A", "D"
	added       int
	deleted     int
	uncommitted string // "", "modified", "untracked", "deleted"
}

type dirNode struct {
	name     string
	children map[string]*dirNode
	files    []diffEntry
}

func newDirNode(name string) *dirNode {
	return &dirNode{name: name, children: make(map[string]*dirNode)}
}

// containerGit runs a git command inside the sandbox container.
// It detects nested worktrees and sets GIT_DIR/GIT_WORK_TREE accordingly.
func containerGit(containerName string, args ...string) ([]byte, error) {
	// Check if the container has a nested worktree (agent created one inside)
	// by looking for .worktrees/ directory
	wtCheck, _ := exec.Command("docker", "exec", containerName,
		"sh", "-c", "ls -d /workspace/.worktrees/*/ 2>/dev/null | head -1").Output()
	wtPath := strings.TrimSpace(string(wtCheck))

	var dockerArgs []string
	if wtPath != "" {
		// Nested worktree: use same GIT_DIR/GIT_WORK_TREE as the agent
		gitCmd := fmt.Sprintf("GIT_DIR=/workspace/.git GIT_WORK_TREE=%s git %s",
			wtPath, strings.Join(args, " "))
		dockerArgs = []string{"exec", containerName, "bash", "-c", gitCmd}
	} else {
		// Normal: just run git from /workspace
		dockerArgs = []string{"exec", containerName, "git", "-C", "/workspace"}
		dockerArgs = append(dockerArgs, args...)
	}
	return exec.Command("docker", dockerArgs...).Output()
}

// buildDiffTree runs git commands inside the container and returns a rendered file tree string.
func buildDiffTree(_ string, sandboxName string) (string, error) {
	containerName := fmt.Sprintf("sc-%s", sandboxName)

	// 1. Count commits on this branch
	commitOut, _ := containerGit(containerName, "rev-list", "--count", "main..HEAD")
	commitCount, _ := strconv.Atoi(strings.TrimSpace(string(commitOut)))

	// 2. Committed changes: diff main..HEAD (what merge will apply)
	committedStatus, err := containerGit(containerName, "diff", "--name-status", "main...HEAD")
	if err != nil {
		return "", fmt.Errorf("git diff main..HEAD: %w", err)
	}
	committedNumstat, _ := containerGit(containerName, "diff", "--numstat", "main...HEAD")

	// 3. Working tree status for uncommitted changes (as the agent sees them)
	porcelainOut, _ := containerGit(containerName, "status", "--porcelain")

	// Build entries map
	entries := make(map[string]*diffEntry)

	// Parse committed --name-status
	for _, line := range strings.Split(strings.TrimSpace(string(committedStatus)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		status := fields[0][:1]
		file := fields[len(fields)-1] // last field handles renames
		entries[file] = &diffEntry{path: file, status: status}
	}

	// Parse committed --numstat
	for _, line := range strings.Split(strings.TrimSpace(string(committedNumstat)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		file := fields[2]
		if e, ok := entries[file]; ok {
			e.added = added
			e.deleted = deleted
		}
	}

	// Overlay porcelain status
	for _, line := range strings.Split(strings.TrimSpace(string(porcelainOut)), "\n") {
		if line == "" {
			continue
		}
		// Porcelain format: "XY filename" — use Fields to robustly extract filename
		// X = index status, Y = worktree status
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		// Extract filename: everything after the "XY " prefix
		// Use strings.Fields for robustness in case of extra whitespace
		rest := strings.TrimSpace(line[2:])
		if rest == "" {
			continue
		}
		// Handle "old -> new" for renames
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			rest = rest[idx+4:]
		}
		file := rest

		switch {
		case x == '?' && y == '?':
			// Untracked file
			if e, exists := entries[file]; exists {
				e.uncommitted = "untracked"
			} else {
				entries[file] = &diffEntry{path: file, status: "A", uncommitted: "untracked"}
			}

		case y == 'D':
			// Deleted in working tree
			if e, exists := entries[file]; exists {
				// File was in committed diff but now deleted locally
				e.uncommitted = "deleted"
			} else {
				entries[file] = &diffEntry{path: file, status: "D", uncommitted: "deleted"}
			}

		case y == 'M' || x == 'M':
			// Modified (staged or unstaged)
			if e, exists := entries[file]; exists {
				e.uncommitted = "modified"
			} else {
				entries[file] = &diffEntry{path: file, status: "M", uncommitted: "modified"}
			}

		case x == 'A':
			// Staged addition
			if e, exists := entries[file]; exists {
				e.uncommitted = "modified"
			} else {
				entries[file] = &diffEntry{path: file, status: "A", uncommitted: "untracked"}
			}
		}
	}

	if len(entries) == 0 {
		return fmt.Sprintf("[%s] No changes yet", sandboxName), nil
	}

	// Sort files
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

	// Render
	var b strings.Builder
	b.WriteString(diffHeaderStyle.Render(sandboxName))
	b.WriteString(diffDimStyle.Render(fmt.Sprintf("  %d commit%s", commitCount, plural(commitCount))))
	b.WriteString("\n")

	renderTree(&b, root, "")

	// Summary
	totalAdd, totalDel, fileCount, uncommittedCount := 0, 0, 0, 0
	for _, e := range entries {
		totalAdd += e.added
		totalDel += e.deleted
		fileCount++
		if e.uncommitted != "" {
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
		b.WriteString(diffWarnStyle.Render(fmt.Sprintf(
			"⚠ %d file%s with uncommitted changes", uncommittedCount, plural(uncommittedCount))))
	}

	return b.String(), nil
}

func renderTree(b *strings.Builder, node *dirNode, prefix string) {
	var dirNames []string
	for name := range node.children {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)

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

	// Uncommitted warning with detail
	switch entry.uncommitted {
	case "untracked":
		badges = append(badges, diffWarnStyle.Render("⚠ untracked"))
	case "deleted":
		badges = append(badges, diffWarnStyle.Render("⚠ locally deleted"))
	case "modified":
		badges = append(badges, diffWarnStyle.Render("⚠ uncommitted changes"))
	}

	// Line counts
	var counts []string
	if entry.added > 0 {
		counts = append(counts, diffAddStyle.Render(fmt.Sprintf("+%d", entry.added)))
	}
	if entry.deleted > 0 {
		counts = append(counts, diffDelStyle.Render(fmt.Sprintf("-%d", entry.deleted)))
	}

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
