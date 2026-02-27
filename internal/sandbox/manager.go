package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

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

// Create spins up a new sandbox: creates a worktree, builds the image, starts a container.
func (m *Manager) Create(name, task string) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.state.Sandboxes[name]; exists {
		return nil, fmt.Errorf("sandbox %q already exists", name)
	}

	// Create git worktree
	wtPath, branch, err := worktree.Create(m.projectDir, name)
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %w", err)
	}

	// Build the Docker image
	if err := m.buildImage(); err != nil {
		worktree.Remove(m.projectDir, name)
		return nil, fmt.Errorf("building image: %w", err)
	}

	// Build docker run args
	containerName := fmt.Sprintf("sc-%s", name)
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-v", fmt.Sprintf("%s:/workspace", wtPath),
	}

	// Mount claude config read-only
	home, _ := os.UserHomeDir()
	claudeDir := home + "/.claude"
	if _, err := os.Stat(claudeDir); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/root/.claude:ro", claudeDir))
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

	// Start tmux session inside container
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
	}
	m.state.Sandboxes[name] = sb
	m.persist()

	return sb, nil
}

// Destroy stops and removes a sandbox container and its worktree.
func (m *Manager) Destroy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	containerName := fmt.Sprintf("sc-%s", name)
	exec.Command("docker", "stop", containerName).Run()
	exec.Command("docker", "rm", containerName).Run()

	worktree.Remove(m.projectDir, name)

	delete(m.state.Sandboxes, name)
	m.persist()
	return nil
}

// ConnectCmd returns an exec.Cmd to attach to a sandbox's tmux session.
func (m *Manager) ConnectCmd(name string) *exec.Cmd {
	containerName := fmt.Sprintf("sc-%s", name)
	return exec.Command("docker", "exec", "-it", containerName, "tmux", "attach-session", "-t", "main")
}

// List returns all sandboxes sorted by name.
func (m *Manager) List() []*Sandbox {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Sandbox, 0, len(m.state.Sandboxes))
	for _, sb := range m.state.Sandboxes {
		result = append(result, sb)
	}
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
	cmd := exec.Command("docker", "build", "-t", m.imageName(), "-f", dockerfilePath, ".")
	cmd.Dir = m.projectDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
