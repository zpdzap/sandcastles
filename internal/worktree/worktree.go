package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zpdzap/sandcastles/internal/config"
)

// Create creates a new git worktree for a sandbox.
// Returns the absolute worktree path and branch name.
func Create(projectDir, name string) (string, string, error) {
	wtPath := filepath.Join(projectDir, config.Dir, config.WorktreeDir, name)
	branch := fmt.Sprintf("sandcastle/%s", name)

	cmd := exec.Command("git", "worktree", "add", wtPath, "-b", branch)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	absPath, err := filepath.Abs(wtPath)
	if err != nil {
		return wtPath, branch, nil
	}
	return absPath, branch, nil
}

// Remove removes a git worktree and optionally deletes the branch.
func Remove(projectDir, name string) error {
	wtPath := filepath.Join(projectDir, config.Dir, config.WorktreeDir, name)
	branch := fmt.Sprintf("sandcastle/%s", name)

	// Remove the worktree
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = projectDir
	cmd.Run() // best-effort

	// Delete the branch
	branchCmd := exec.Command("git", "branch", "-D", branch)
	branchCmd.Dir = projectDir
	branchCmd.Run() // best-effort

	return nil
}

// List returns the names of existing sandcastle worktrees.
func List(projectDir string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	prefix := filepath.Join(projectDir, config.Dir, config.WorktreeDir)
	var names []string

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			if strings.HasPrefix(path, prefix) {
				name := filepath.Base(path)
				names = append(names, name)
			}
		}
	}
	return names, nil
}
