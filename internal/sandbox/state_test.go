package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zpdzap/sandcastles/internal/config"
)

func TestStateLoadSave(t *testing.T) {
	dir := t.TempDir()
	// Create .sandcastles dir
	os.MkdirAll(filepath.Join(dir, config.Dir), 0o755)

	state := newState()
	state.Sandboxes["test"] = &Sandbox{
		Name:         "test",
		ContainerID:  "abc123",
		Status:       StatusRunning,
		Task:         "fix a bug",
		Branch:       "sandcastle/test",
		WorktreePath: "/tmp/test",
		Ports:        map[string]string{"3000": "49321"},
	}

	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	loaded, err := loadState(dir)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}

	sb, ok := loaded.Sandboxes["test"]
	if !ok {
		t.Fatal("sandbox 'test' not found in loaded state")
	}
	if sb.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", sb.ContainerID, "abc123")
	}
	if sb.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", sb.Status, StatusRunning)
	}
	if sb.Ports["3000"] != "49321" {
		t.Errorf("Ports[3000] = %q, want %q", sb.Ports["3000"], "49321")
	}
}

func TestStateLoadMissing(t *testing.T) {
	dir := t.TempDir()
	state, err := loadState(dir)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if len(state.Sandboxes) != 0 {
		t.Errorf("expected empty state, got %d sandboxes", len(state.Sandboxes))
	}
}
