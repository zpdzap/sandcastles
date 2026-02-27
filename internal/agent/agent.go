package agent

import (
	"fmt"
	"os/exec"
)

// Start launches Claude Code inside a sandbox's tmux session using send-keys.
// This is non-fatal — if it fails, the container is still usable manually.
func Start(containerName, task string) error {
	if task == "" {
		return nil
	}

	// Use tmux send-keys to type the claude command into the session
	claudeCmd := fmt.Sprintf("claude --dangerously-skip-permissions %q", task)

	cmd := exec.Command("docker", "exec", containerName,
		"tmux", "send-keys", "-t", "main", claudeCmd, "Enter")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("agent start failed: %s: %w", string(out), err)
	}
	return nil
}
