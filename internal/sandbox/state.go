package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zpdzap/sandcastles/internal/config"
)

// State holds the persistent sandbox state.
type State struct {
	Sandboxes map[string]*Sandbox `json:"sandboxes"`
}

func newState() *State {
	return &State{Sandboxes: make(map[string]*Sandbox)}
}

func statePath(projectDir string) string {
	return filepath.Join(projectDir, config.Dir, config.StateFile)
}

func loadState(projectDir string) (*State, error) {
	path := statePath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newState(), nil
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	if s.Sandboxes == nil {
		s.Sandboxes = make(map[string]*Sandbox)
	}
	return &s, nil
}

func saveState(projectDir string, s *State) error {
	dir := filepath.Join(projectDir, config.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	return os.WriteFile(statePath(projectDir), data, 0o644)
}
