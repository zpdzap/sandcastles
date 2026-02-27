package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zpdzap/sandcastles/internal/config"
	"github.com/zpdzap/sandcastles/internal/worktree"
)

// Manager handles Docker container lifecycle and persistent state.
type Manager struct {
	mu         sync.Mutex
	projectDir string
	cfg        *config.Config
	state      *State
}

// NewManager creates a new sandbox manager.
func NewManager(projectDir string, cfg *config.Config) *Manager {
	state, err := loadState(projectDir)
	if err != nil {
		state = newState()
	}
	return &Manager{
		projectDir: projectDir,
		cfg:        cfg,
		state:      state,
	}
}

// ProgressFunc is called with status updates during sandbox creation.
type ProgressFunc func(phase string)

// Create spins up a new sandbox: creates a worktree, builds the image, starts a container.
// If progress is non-nil, it's called with phase updates.
func (m *Manager) Create(name, task string, progress ProgressFunc) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := func(phase string) {
		if progress != nil {
			progress(phase)
		}
	}

	if _, exists := m.state.Sandboxes[name]; exists {
		return nil, fmt.Errorf("sandbox %q already exists", name)
	}

	// Create git worktree
	report("Creating worktree...")
	wtPath, branch, err := worktree.Create(m.projectDir, name)
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %w", err)
	}

	// Build the Docker image
	report("Building image (may take a minute on first run)...")
	if err := m.buildImage(); err != nil {
		worktree.Remove(m.projectDir, name)
		return nil, fmt.Errorf("building image: %w", err)
	}

	// Start container
	report("Starting container...")
	containerName := fmt.Sprintf("sc-%s", name)
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-v", fmt.Sprintf("%s:/workspace", wtPath),
	}

	// Port mappings
	for _, port := range m.cfg.Defaults.Ports {
		args = append(args, "-p", fmt.Sprintf("0:%d", port))
	}

	// Environment variables
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		args = append(args, "-e", "ANTHROPIC_API_KEY")
	}
	for k, v := range m.cfg.Defaults.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Extra mounts
	for _, mount := range m.cfg.Defaults.Mounts {
		args = append(args, "-v", mount)
	}

	args = append(args, m.imageName(), "sleep", "infinity")

	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		worktree.Remove(m.projectDir, name)
		return nil, fmt.Errorf("docker run failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	containerID := strings.TrimSpace(string(out))
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}

	// Copy claude config files into container (writable, owned by sandcastle user)
	report("Configuring Claude Code...")
	home, _ := os.UserHomeDir()

	// Files inside ~/.claude/
	claudeDirFiles := []string{
		"settings.json",
		".credentials.json",
	}
	// Ensure .claude dir exists in container
	exec.Command("docker", "exec", containerName, "mkdir", "-p", "/home/sandcastle/.claude").Run()
	for _, f := range claudeDirFiles {
		hostPath := home + "/.claude/" + f
		containerPath := "/home/sandcastle/.claude/" + f
		if _, err := os.Stat(hostPath); err == nil {
			exec.Command("docker", "cp", hostPath, containerName+":"+containerPath).Run()
			exec.Command("docker", "exec", "--user", "root", containerName,
				"chown", "sandcastle:sandcastle", containerPath).Run()
		}
	}

	// ~/.claude.json (preferences — lives at home root, not inside .claude/)
	// Copy it, then patch to pre-trust /workspace and mark onboarding complete
	claudeJSON := home + "/.claude.json"
	if _, err := os.Stat(claudeJSON); err == nil {
		exec.Command("docker", "cp", claudeJSON, containerName+":/home/sandcastle/.claude.json").Run()
		exec.Command("docker", "exec", "--user", "root", containerName,
			"chown", "sandcastle:sandcastle", "/home/sandcastle/.claude.json").Run()
	}
	// Patch .claude.json to pre-trust /workspace and skip onboarding
	patchScript := `python3 -c "
import json, os
p = '/home/sandcastle/.claude.json'
try:
    d = json.load(open(p))
except:
    d = {}
d['hasCompletedOnboarding'] = True
d.setdefault('projects', {})['/workspace'] = {
    'allowedTools': [],
    'hasTrustDialogAccepted': True,
    'hasCompletedProjectOnboarding': True,
}
json.dump(d, open(p, 'w'))
"`
	exec.Command("docker", "exec", containerName, "bash", "-c", patchScript).Run()

	// Patch settings.json to default to bypass permissions (no interactive confirmation)
	settingsPatch := `python3 -c "
import json
p = '/home/sandcastle/.claude/settings.json'
try:
    d = json.load(open(p))
except:
    d = {}
d['defaultMode'] = 'bypassPermissions'
json.dump(d, open(p, 'w'))
"`
	exec.Command("docker", "exec", containerName, "bash", "-c", settingsPatch).Run()

	// Start tmux session inside container
	report("Starting tmux session...")
	tmuxCmd := exec.Command("docker", "exec", "-d", containerName, "tmux", "new-session", "-d", "-s", "main")
	if err := tmuxCmd.Run(); err != nil {
		// Non-fatal — container is still usable
		fmt.Fprintf(os.Stderr, "Warning: tmux start failed: %v\n", err)
	}

	// Query port mappings
	ports := m.queryPorts(containerName)

	sb := &Sandbox{
		Name:         name,
		ContainerID:  containerID,
		Status:       StatusRunning,
		Task:         task,
		Branch:       branch,
		WorktreePath: wtPath,
		Ports:        ports,
		CreatedAt:    time.Now(),
	}
	m.state.Sandboxes[name] = sb
	m.persist()

	return sb, nil
}

// MarkStopping sets a sandbox to "stopping" status so the TUI shows feedback immediately.
func (m *Manager) MarkStopping(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sb, ok := m.state.Sandboxes[name]; ok {
		sb.Status = StatusStopping
	}
}

// Destroy stops and removes a sandbox container and its worktree.
func (m *Manager) Destroy(name string) error {
	containerName := fmt.Sprintf("sc-%s", name)

	// Slow Docker operations — run WITHOUT holding the lock so TUI doesn't freeze
	exec.Command("docker", "stop", containerName).Run()
	exec.Command("docker", "rm", containerName).Run()
	worktree.Remove(m.projectDir, name)

	// Now grab the lock briefly to update state
	m.mu.Lock()
	delete(m.state.Sandboxes, name)
	m.persist()
	m.mu.Unlock()

	return nil
}

// ConnectCmd returns an exec.Cmd to attach to a sandbox's tmux session.
func (m *Manager) ConnectCmd(name string) *exec.Cmd {
	containerName := fmt.Sprintf("sc-%s", name)
	return exec.Command("docker", "exec", "-it", containerName, "tmux", "attach-session", "-t", "main")
}

// List returns all sandboxes sorted by creation time.
func (m *Manager) List() []*Sandbox {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Sandbox, 0, len(m.state.Sandboxes))
	for _, sb := range m.state.Sandboxes {
		result = append(result, sb)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// Get returns a sandbox by name.
func (m *Manager) Get(name string) (*Sandbox, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sb, ok := m.state.Sandboxes[name]
	return sb, ok
}

// Reconcile syncs the state file with actual Docker container states.
func (m *Manager) Reconcile() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	for name, sb := range m.state.Sandboxes {
		containerName := fmt.Sprintf("sc-%s", name)
		status := inspectStatus(containerName)

		if status == "" {
			// Container doesn't exist — remove from state
			delete(m.state.Sandboxes, name)
			changed = true
			continue
		}

		newStatus := dockerToStatus(status)
		if sb.Status != newStatus {
			sb.Status = newStatus
			changed = true
		}

		// Refresh port mappings for running containers
		if newStatus == StatusRunning {
			sb.Ports = m.queryPorts(containerName)
		}
	}

	if changed {
		m.persist()
	}
	return nil
}

// RefreshStatuses polls Docker for current container statuses.
func (m *Manager) RefreshStatuses() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, sb := range m.state.Sandboxes {
		// Don't overwrite transient states managed by the TUI
		if sb.Status == StatusStopping {
			continue
		}
		containerName := fmt.Sprintf("sc-%s", name)
		status := inspectStatus(containerName)
		if status == "" {
			sb.Status = StatusStopped
		} else {
			sb.Status = dockerToStatus(status)
		}
	}
}

// DestroyAll destroys all sandboxes.
func (m *Manager) DestroyAll() {
	// Collect names first (Destroy takes the lock)
	m.mu.Lock()
	names := make([]string, 0, len(m.state.Sandboxes))
	for name := range m.state.Sandboxes {
		names = append(names, name)
	}
	m.mu.Unlock()

	for _, name := range names {
		m.Destroy(name)
	}
}

func (m *Manager) imageName() string {
	return fmt.Sprintf("sc-%s", m.cfg.Project)
}

func (m *Manager) buildImage() error {
	dockerfilePath := m.cfg.Image.Dockerfile
	cmd := exec.Command("docker", "build", "-q", "-t", m.imageName(), "-f", dockerfilePath, ".")
	cmd.Dir = m.projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (m *Manager) queryPorts(containerName string) map[string]string {
	ports := make(map[string]string)
	out, err := exec.Command("docker", "port", containerName).CombinedOutput()
	if err != nil {
		return ports
	}
	// Parse lines like: "3000/tcp -> 0.0.0.0:49321"
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " -> ", 2)
		if len(parts) != 2 {
			continue
		}
		containerPort := strings.SplitN(parts[0], "/", 2)[0]
		hostParts := strings.SplitN(parts[1], ":", 2)
		if len(hostParts) == 2 {
			ports[containerPort] = hostParts[1]
		}
	}
	return ports
}

func (m *Manager) persist() {
	if err := saveState(m.projectDir, m.state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
	}
}

func inspectStatus(containerName string) string {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", containerName).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func dockerToStatus(dockerStatus string) Status {
	switch dockerStatus {
	case "running":
		return StatusRunning
	case "exited", "dead":
		return StatusStopped
	case "created", "restarting":
		return StatusCreating
	default:
		return StatusError
	}
}
