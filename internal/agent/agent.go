package agent

import (
	"fmt"
	"os/exec"
	"time"
)

// Start launches Claude Code inside a sandbox's tmux session using send-keys.
// Wraps claude with claude-chill to prevent terminal flicker from large redraws.
// If task is provided, passes it as the initial prompt. Otherwise starts Claude interactively.
// This is non-fatal — if it fails, the container is still usable manually.
func Start(containerName, task string) error {
	// Brief pause to let the tmux session fully initialize
	time.Sleep(500 * time.Millisecond)

	// Use tmux send-keys to type the claude command into the session.
	// claude-chill is a PTY proxy that intercepts Claude's massive sync block
	// redraws and sends only diffs, eliminating flicker in tmux.
	claudeCmd := "claude-chill claude"
	if task != "" {
		claudeCmd = fmt.Sprintf("claude-chill -- claude %q", task)
	}

	cmd := exec.Command("docker", "exec", containerName,
		"tmux", "send-keys", "-t", "main", claudeCmd, "Enter")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("agent start failed: %s: %w", string(out), err)
	}
	return nil
}
