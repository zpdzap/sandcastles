package sandbox

// Status represents the current state of a sandbox container.
type Status string

const (
	StatusCreating Status = "creating"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
	StatusError    Status = "error"
)

// Sandbox represents a single sandboxed container with its associated worktree.
type Sandbox struct {
	Name         string            `json:"name"`
	ContainerID  string            `json:"container_id"`
	Status       Status            `json:"status"`
	Task         string            `json:"task"`
	Branch       string            `json:"branch"`
	WorktreePath string            `json:"worktree_path"`
	Ports        map[string]string `json:"ports"` // container port → host port
}
