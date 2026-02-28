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
					Agent:        "claude",
					Ports:        detection.Ports,
					Env:          map[string]string{},
					Mounts:       nil,
					DockerSocket: detection.DockerSocket,
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
			if detection.DockerSocket {
				fmt.Println("\n  Detected docker-compose files — docker_socket enabled.")
				fmt.Println("  If your tests need localhost access to containers, add to config.yaml:")
				fmt.Println("    defaults:")
				fmt.Println("      network: host")
			}
			fmt.Println("\nRun `sc` to launch the dashboard.")
			return nil
		},
	}
}

func writeDockerfile(projectDir string, cfg *config.Config) error {
	// Remove nodejs/npm from apt packages — we install Node via NodeSource
	// to get a modern version (Ubuntu's nodejs is too old for most tools)
	var pkgs []string
	for _, p := range cfg.Image.Packages {
		if p != "nodejs" && p != "npm" {
			pkgs = append(pkgs, p)
		}
	}
	// Ensure ca-certificates and gnupg are present (needed for NodeSource)
	for _, needed := range []string{"ca-certificates", "gnupg"} {
		found := false
		for _, p := range pkgs {
			if p == needed {
				found = true
				break
			}
		}
		if !found {
			pkgs = append(pkgs, needed)
		}
	}
	// Docker CLI for docker socket support
	if cfg.Defaults.DockerSocket {
		hasDocker := false
		for _, p := range pkgs {
			if p == "docker.io" {
				hasDocker = true
				break
			}
		}
		if !hasDocker {
			pkgs = append(pkgs, "docker.io")
		}
	}
	packages := strings.Join(pkgs, " ")

	userGroups := "sudo"
	if cfg.Defaults.DockerSocket {
		userGroups = "sudo,docker"
	}

	dockerCompose := ""
	if cfg.Defaults.DockerSocket {
		dockerCompose = `
# Install Docker Compose v2 plugin
RUN mkdir -p /usr/local/lib/docker/cli-plugins && \
    curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$(uname -m)" \
    -o /usr/local/lib/docker/cli-plugins/docker-compose && \
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
`
	}

	content := fmt.Sprintf(`FROM %s

ARG HOST_UID=1000
ARG HOST_GID=1000

RUN apt-get update && apt-get install -y \
    tmux \
    %s \
    sudo \
    locales \
    && rm -rf /var/lib/apt/lists/* \
    && locale-gen en_US.UTF-8

ENV LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 TERM=xterm-256color

# Install Node.js 22 from NodeSource (Ubuntu's nodejs is too old)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*
%s
RUN npm install -g @anthropic-ai/claude-code

# Non-root user with host UID/GID so bind-mounted files are writable
# If UID/GID already exist (e.g. ubuntu user), take them over
RUN existing_user=$(getent passwd $HOST_UID | cut -d: -f1) && \
    if [ -n "$existing_user" ] && [ "$existing_user" != "sandcastle" ]; then \
        usermod -l sandcastle -d /home/sandcastle -m "$existing_user" && \
        existing_group=$(getent group $HOST_GID | cut -d: -f1) && \
        if [ -n "$existing_group" ] && [ "$existing_group" != "sandcastle" ]; then \
            groupmod -n sandcastle "$existing_group"; \
        fi; \
    else \
        groupadd -g $HOST_GID sandcastle 2>/dev/null; \
        useradd -m -s /bin/bash -u $HOST_UID -g $HOST_GID sandcastle; \
    fi && \
    usermod -aG %s -s /bin/bash sandcastle && \
    echo 'sandcastle ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Go tools on PATH
ENV PATH="/home/sandcastle/go/bin:${PATH}"

RUN mkdir -p /workspace && chown sandcastle:sandcastle /workspace
WORKDIR /workspace

USER sandcastle

RUN echo 'set -g mouse on' > ~/.tmux.conf && \
    echo 'set -g status-style "bg=#1a1a2e,fg=#FFD700"' >> ~/.tmux.conf && \
    echo 'set -g status-left " sandcastle "' >> ~/.tmux.conf && \
    echo 'set -g status-right " %%H:%%M "' >> ~/.tmux.conf

CMD ["sleep", "infinity"]
`, cfg.Image.Base, packages, dockerCompose, userGroups)

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
