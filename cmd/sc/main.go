package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zpdzap/sandcastles/internal/config"
	"github.com/zpdzap/sandcastles/internal/sandbox"
	"github.com/zpdzap/sandcastles/internal/tui"
)

func main() {
	root := &cobra.Command{
		Use:   "sc",
		Short: "Sandcastles — orchestrate AI coding agents in isolated containers",
		RunE:  runTUI,
	}

	root.AddCommand(initCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize sandcastles in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir, err := os.Getwd()
			if err != nil {
				return err
			}

			if config.Exists(projectDir) {
				fmt.Println("Sandcastles already initialized in this project.")
				return nil
			}

			detection := config.Detect(projectDir)
			projectName := filepath.Base(projectDir)

			cfg := &config.Config{
				Version:  "1",
				Project:  projectName,
				Language: detection.Language,
				Image: config.Image{
					Base:       "ubuntu:24.04",
					Dockerfile: ".sandcastles/Dockerfile",
					Packages:   detection.Packages,
				},
				Defaults: config.Defaults{
					Agent:  "claude",
					Ports:  detection.Ports,
					Env:    map[string]string{},
					Mounts: nil,
				},
			}

			if err := config.Save(projectDir, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			if err := writeDockerfile(projectDir, cfg); err != nil {
				return fmt.Errorf("writing Dockerfile: %w", err)
			}

			if err := updateGitignore(projectDir); err != nil {
				return fmt.Errorf("updating .gitignore: %w", err)
			}

			// Create worktrees directory
			wtDir := filepath.Join(projectDir, config.Dir, config.WorktreeDir)
			if err := os.MkdirAll(wtDir, 0o755); err != nil {
				return fmt.Errorf("creating worktrees dir: %w", err)
			}

			fmt.Printf("Initialized sandcastles for %s (%s project)\n", projectName, detection.Language)
			fmt.Printf("  Config: %s/%s\n", config.Dir, config.ConfigFile)
			fmt.Printf("  Dockerfile: %s/Dockerfile\n", config.Dir)
			fmt.Println("\nRun `sc` to launch the dashboard.")
			return nil
		},
	}
}

func writeDockerfile(projectDir string, cfg *config.Config) error {
	// Ensure npm is always available (needed for Claude Code)
	pkgs := cfg.Image.Packages
	hasNpm := false
	for _, p := range pkgs {
		if p == "npm" {
			hasNpm = true
			break
		}
	}
	if !hasNpm {
		pkgs = append(pkgs, "npm")
	}
	packages := strings.Join(pkgs, " ")

	content := fmt.Sprintf(`FROM %s

RUN apt-get update && apt-get install -y \
    tmux \
    %s \
    sudo \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

# Non-root user (Claude Code refuses --dangerously-skip-permissions as root)
RUN useradd -m -s /bin/bash -G sudo sandcastle && \
    echo 'sandcastle ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

RUN mkdir -p /workspace && chown sandcastle:sandcastle /workspace
WORKDIR /workspace

USER sandcastle

RUN echo 'set -g mouse on' > ~/.tmux.conf && \
    echo 'set -g status-style "bg=#1a1a2e,fg=#FFD700"' >> ~/.tmux.conf && \
    echo 'set -g status-left " sandcastle "' >> ~/.tmux.conf && \
    echo 'set -g status-right " %%H:%%M "' >> ~/.tmux.conf

CMD ["sleep", "infinity"]
`, cfg.Image.Base, packages)

	path := filepath.Join(projectDir, config.Dir, "Dockerfile")
	return os.WriteFile(path, []byte(content), 0o644)
}

func updateGitignore(projectDir string) error {
	gitignorePath := filepath.Join(projectDir, ".gitignore")

	entries := []string{
		".sandcastles/worktrees/",
		".sandcastles/state.json",
	}

	existing, _ := os.ReadFile(gitignorePath)
	content := string(existing)

	var toAdd []string
	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	content += "\n# sandcastles\n"
	for _, entry := range toAdd {
		content += entry + "\n"
	}

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}

func runTUI(cmd *cobra.Command, args []string) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return fmt.Errorf("not a sandcastles project (run `sc init` first): %w", err)
	}

	mgr := sandbox.NewManager(projectDir, cfg)

	// Reconcile state with actual Docker containers on startup
	if err := mgr.Reconcile(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: state reconciliation failed: %v\n", err)
	}

	return tui.Run(mgr, cfg)
}
