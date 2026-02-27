package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Version:  "1",
		Project:  "test-project",
		Language: "go",
		Image: Image{
			Base:       "ubuntu:24.04",
			Dockerfile: ".sandcastles/Dockerfile",
			Packages:   []string{"golang-go"},
		},
		Defaults: Defaults{
			Agent:  "claude",
			Ports:  []int{8080},
			Env:    map[string]string{},
			Mounts: nil,
		},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Project != "test-project" {
		t.Errorf("Project = %q, want %q", loaded.Project, "test-project")
	}
	if loaded.Language != "go" {
		t.Errorf("Language = %q, want %q", loaded.Language, "go")
	}
	if len(loaded.Defaults.Ports) != 1 || loaded.Defaults.Ports[0] != 8080 {
		t.Errorf("Ports = %v, want [8080]", loaded.Defaults.Ports)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir) {
		t.Error("Exists should be false before init")
	}

	cfg := &Config{Version: "1", Project: "test"}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !Exists(dir) {
		t.Error("Exists should be true after save")
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		wantLang string
	}{
		{"go project", "go.mod", "go"},
		{"node project", "package.json", "node"},
		{"python project", "requirements.txt", "python"},
		{"rust project", "Cargo.toml", "rust"},
		{"unknown project", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.file != "" {
				os.WriteFile(filepath.Join(dir, tt.file), []byte(""), 0o644)
			}
			d := Detect(dir)
			if d.Language != tt.wantLang {
				t.Errorf("Language = %q, want %q", d.Language, tt.wantLang)
			}
		})
	}
}
