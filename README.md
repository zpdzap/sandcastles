# sandcastles

Orchestrate multiple AI coding agents in isolated Docker containers. Each agent gets its own git worktree volume-mounted into a container — full process/network isolation, but code changes visible in your IDE in real-time.

## Install

```bash
# Homebrew
brew install zpdzap/tap/sandcastles

# From source
go install github.com/zpdzap/sandcastles/cmd/sc@latest
```

## Prerequisites

- Docker
- Git
- Go 1.24+

## Quick Start

```bash
# Initialize in your project
cd your-project
sc init

# Launch the TUI dashboard
sc
```

## TUI Commands

| Command | Description |
|---------|-------------|
| `/start <name> [task]` | Create a sandbox with optional task for the AI agent |
| `/stop <name>` | Stop and remove a sandbox |
| `/connect <name>` | Attach to a sandbox's tmux session |
| `/diff <name>` | Show git diff from a sandbox's worktree |
| `/quit` | Shut down all sandboxes and exit |

## How It Works

1. **`sc init`** detects your project language and generates `.sandcastles/config.yaml` + a Dockerfile
2. **`/start`** creates a git worktree, builds a Docker image, starts a container with the worktree mounted at `/workspace`
3. If a task is provided, Claude Code auto-starts inside the container's tmux session
4. **Enter** on a sandbox drops you into the tmux session (detach with `Ctrl-B d`)
5. Code changes appear in `.sandcastles/worktrees/<name>/` — open it in your IDE
6. **`/stop`** cleans up the container, worktree, and branch

## Config

`.sandcastles/config.yaml`:

```yaml
version: "1"
project: my-app
language: go
image:
  base: ubuntu:24.04
  dockerfile: .sandcastles/Dockerfile
  packages: [golang-go, git, curl]
defaults:
  agent: claude
  ports: [8080]
  env: {}
  mounts: []
```

Ports listed in `defaults.ports` are auto-mapped to random host ports. The dashboard shows the mapping.

## License

MIT
