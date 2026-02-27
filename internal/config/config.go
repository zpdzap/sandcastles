package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	Dir        = ".sandcastles"
	ConfigFile = "config.yaml"
	StateFile  = "state.json"
	WorktreeDir = "worktrees"
)

type Config struct {
	Version  string   `yaml:"version"`
	Project  string   `yaml:"project"`
	Language string   `yaml:"language"`
	Image    Image    `yaml:"image"`
	Defaults Defaults `yaml:"defaults"`
}

type Image struct {
	Base       string   `yaml:"base"`
	Dockerfile string   `yaml:"dockerfile"`
	Packages   []string `yaml:"packages"`
}

type Defaults struct {
	Agent        string            `yaml:"agent"`
	Ports        []int             `yaml:"ports"`
	Env          map[string]string `yaml:"env"`
	Mounts       []string          `yaml:"mounts"`
	Network      string            `yaml:"network,omitempty"`
	DockerSocket bool              `yaml:"docker_socket,omitempty"`
}

// IsHostNetwork returns true if the sandbox should use host networking.
func (d Defaults) IsHostNetwork() bool { return d.Network == "host" }

// Load reads config from .sandcastles/config.yaml relative to projectDir.
func Load(projectDir string) (*Config, error) {
	path := filepath.Join(projectDir, Dir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes config to .sandcastles/config.yaml relative to projectDir.
func Save(projectDir string, cfg *Config) error {
	dir := filepath.Join(projectDir, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	path := filepath.Join(dir, ConfigFile)
	return os.WriteFile(path, data, 0o644)
}

// ConfigPath returns the path to the config directory.
func ConfigPath(projectDir string) string {
	return filepath.Join(projectDir, Dir)
}

// Exists returns true if .sandcastles/config.yaml exists.
func Exists(projectDir string) bool {
	path := filepath.Join(projectDir, Dir, ConfigFile)
	_, err := os.Stat(path)
	return err == nil
}
