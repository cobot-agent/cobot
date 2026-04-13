# User Guide - Getting Started with Cobot

## Overview

Cobot is a multi-workspace AI agent framework. Each workspace is an isolated environment with its own persona, memory, agents, and tool configuration. Workspaces can be scoped to a project directory or used as general-purpose environments. Multiple agents with different models and capabilities can live within a single workspace.

## Requirements

- Go 1.26 or later
- Git

## Installation

### From Source

```bash
git clone https://github.com/cobot-agent/cobot.git
cd cobot

# Build
make build

# Install to $GOPATH/bin
make install

# Install system-wide
make install-system
```

### Quick Install (macOS/Linux)

```bash
curl -sSL https://raw.githubusercontent.com/cobot-agent/cobot/main/install.sh | bash
```

## First-Time Setup

### 1. Run Setup

```bash
cobot setup
```

`cobot setup` initializes the system:

- Creates the config directory (`~/.config/cobot/` by default)
- Creates the data directory (`~/.local/share/cobot/` by default)
- Creates a `default` workspace under `~/.local/share/cobot/default/`
- Generates starter persona files (SOUL.md, USER.md, MEMORY.md) in the default workspace
- Writes `~/.config/cobot/config.yaml`

### 2. Configure API Keys

```bash
# Set OpenAI key
cobot config set api_keys.openai sk-your-key-here

# Set Anthropic key
cobot config set api_keys.anthropic sk-your-key-here
```

Alternatively, use environment variables (see [Environment Variables](#environment-variables)).

### 3. Verify Health

```bash
cobot doctor
```

Expected output:

```
Cobot Doctor
============

Config directory: /Users/you/.config/cobot
  [OK] Directory exists
  [OK] Config file: /Users/you/.config/cobot/config.yaml
  [OK] Model: openai:gpt-4o
  [OK] API keys configured: [openai]

Workspace: default
  [OK] Data directory: /Users/you/.local/share/cobot/default
  [OK] workspace.yaml
  [OK] SOUL.md
  [OK] USER.md
  [OK] MEMORY.md

All critical checks passed!
```

## Directory Structure

Cobot separates **system-level configuration** (agent-immutable) from **workspace data** (agent-mutable).

```
~/.config/cobot/                          # Config dir — agent-immutable
├── config.yaml                           # Global settings (API keys, default model)
├── mcp/                                  # MCP server registry
│   ├── github.yaml
│   └── filesystem.yaml
├── skills/                               # Global skills registry
│   ├── code-review.yaml
│   ├── debugging.md
│   └── refactor/
│       ├── SKILL.md
│       └── scripts/
└── workspaces/                           # Workspace definitions (name → path mappings)
    ├── default.yaml
    └── my-project.yaml

~/.local/share/cobot/                     # Data dir — workspace data lives here
├── default/                              # Default workspace
│   ├── workspace.yaml                    # Workspace runtime config (agent-mutable)
│   ├── SOUL.md                           # Agent personality (agent-mutable)
│   ├── USER.md                           # User profile (agent-mutable)
│   ├── MEMORY.md                         # Consolidated memory (agent-mutable)
│   ├── agents/                           # Agent definitions
│   │   └── main.yaml
│   ├── skills/                           # Workspace-private skills
│   ├── memory/                           # MemPalace storage (BadgerDB + Bleve)
│   ├── sessions/                         # Chat session data
│   └── scheduler/                        # Periodic summarization tasks
└── my-project/                           # Another workspace
    └── ...
```

The `<config_dir>` is managed exclusively by CLI commands — agents cannot modify it. The `<data_dir>` and everything inside workspace directories is mutable by agents through dedicated tools.

## Workspace Management

### Default Workspace

A `default` workspace is created automatically by `cobot setup`. It is used whenever no workspace is specified.

### Creating a Workspace

```bash
cobot workspace create my-project
```

This registers `my-project` in `~/.config/cobot/workspaces/my-project.yaml` and initializes its data directory at `~/.local/share/cobot/my-project/`.

### Project Workspaces

For a code project, you can associate a workspace with a directory by placing a `.cobot/` folder at the project root:

```
my-project/
├── .cobot/
│   ├── workspace.yaml    # Contains: name: my-project
│   └── AGENTS.md         # Project-level agent instructions
├── src/
└── ...
```

When you run `cobot` from within `my-project/` (or any subdirectory), it walks up the directory tree and discovers `.cobot/workspace.yaml` automatically.

### Workspace Selection

Workspace is determined at runtime — there is no persistent "current workspace" setting. The resolution order is:

| Priority | Method | Example |
|----------|--------|---------|
| 1 (highest) | CLI flag | `cobot chat -w my-project "hello"` |
| 2 | Environment variable | `COBOT_WORKSPACE=my-project` |
| 3 | Project discovery | Walk up from CWD, find `.cobot/workspace.yaml` |
| 4 | Default | `default` workspace |

### Listing and Inspecting Workspaces

```bash
cobot workspace list

cobot workspace show
cobot workspace show my-project
```

### Deleting a Workspace

```bash
cobot workspace delete my-project
```

This removes the workspace definition from the registry. Workspace data in the data directory is not deleted automatically.

## MCP Server Management

MCP servers are defined globally in `~/.config/cobot/mcp/` and selectively enabled per workspace via `enabled_mcp` in `workspace.yaml`.

### Adding an MCP Server

```bash
# Register a new MCP server from a YAML file
cobot mcp add github -f github.yaml
```

Example MCP definition (`github.yaml`):

```yaml
name: github
description: GitHub API via MCP
transport: command
command: npx
args:
  - "@modelcontextprotocol/server-github"
env:
  GITHUB_TOKEN: ${GITHUB_PERSONAL_TOKEN}
```

HTTP transport:

```yaml
name: remote-api
description: Remote API server
transport: http
url: http://localhost:8080
headers:
  Authorization: Bearer ${API_TOKEN}
```

### Managing MCP Servers

```bash
cobot mcp list              # List all registered servers
cobot mcp show github       # Show server definition
cobot mcp test github       # Test connectivity
cobot mcp remove github     # Unregister server
```

### Enabling MCP in a Workspace

Edit the workspace config or use the agent to enable it. In `workspace.yaml`:

```yaml
enabled_mcp:
  - github
  - filesystem
```

Only tools from enabled MCP servers are available to agents in that workspace.

## Skills Management

Skills are reusable instructions or workflows that agents can invoke. They come in three formats and can exist at two scopes.

### Skill Formats

**Single-file YAML:**

```yaml
name: code-review
description: Review code changes
trigger: review
steps:
  - prompt: "Review the following code:\n{{input}}"
    tool: filesystem_read
    output: review_result
```

**Single-file Markdown** (with YAML frontmatter):

```markdown
---
name: debugging
description: Systematic debugging approach
trigger: debug
---

# Debugging Skill

When encountering a bug:
1. Reproduce the issue
2. Isolate the problem
3. Identify root cause
4. Propose fix
```

**Directory form:**

```
refactor/
├── SKILL.md        # Frontmatter + content
└── scripts/        # Optional auxiliary files
```

### Adding Skills to the Global Registry

```bash
cobot skill add code-review -f code-review.yaml
cobot skill add debugging -f debugging.md
```

Global skills are stored in `~/.config/cobot/skills/` and are available to any workspace that enables them.

### Workspace-Private Skills

Skills in `<data_dir>/<workspace>/skills/` are always available to that workspace without needing to be listed in `enabled_skills`. They are agent-mutable: agents can create and update them via the `skill_create` and `skill_update` tools.

If a workspace-private skill has the same name as a global skill, the workspace-private skill takes precedence.

### Managing Skills

```bash
cobot skill list                    # List all global skills
cobot skill list --workspace        # List workspace-private skills
cobot skill show code-review        # Show skill definition
cobot skill remove code-review      # Remove from global registry
```

### Enabling Skills in a Workspace

In `workspace.yaml`:

```yaml
enabled_skills:
  - code-review
  - debugging
```

## Multi-Agent Configuration

Each workspace can define multiple agents, each with a different model, system prompt, and tool set.

### Default Agent

Every workspace has a `default_agent` specified in `workspace.yaml`. When you run `cobot chat`, the default agent is used unless you specify otherwise.

### Agent Definition

Agent configs live in `<data_dir>/<workspace>/agents/<name>.yaml`:

```yaml
name: main
model: openai:gpt-4o
system_prompt: SOUL.md
enabled_mcp:
  - github
  - filesystem
enabled_skills:
  - code-review
  - debugging
max_turns: 50
sandbox: {}              # Empty = inherit workspace sandbox
```

A specialized reviewer agent:

```yaml
name: reviewer
model: anthropic:claude-sonnet-4
system_prompt: |
  You are a code reviewer. Focus on quality, security, and performance.
  Be concise and actionable.
enabled_mcp:
  - github
enabled_skills:
  - code-review
max_turns: 20
sandbox: {}
```

### Listing and Inspecting Agents

```bash
cobot agent list               # List agents in the active workspace
cobot agent show main          # Show agent config
cobot agent show reviewer
```

## Persona Files

Each workspace maintains three persona files in its data directory. Agents can update these files during interactions.

### SOUL.md — Agent Personality

Defines how the agent communicates. Edit it to customize tone, style, and behavior:

```markdown
# SOUL

You are Cobot, an AI assistant for this workspace.

## Voice
- Concise and direct
- Technical but accessible
- Use analogies when helpful

## Style
- Prefer code examples over prose
- Always suggest best practices
- Ask clarifying questions when ambiguous
```

### USER.md — User Profile

Contains information about the user. The agent updates this as it learns about you:

```markdown
# USER

## Profile
- Name: Alex
- Role: Senior Software Engineer
- Languages: Go, TypeScript, Python

## Preferences
- Editor: VS Code
- Values: Clean code, performance, testing

## Work Style
- Test-driven development
- Prefers practical, actionable solutions
```

### MEMORY.md — Consolidated Memory

Updated automatically by the workspace scheduler through periodic summarization. Contains:

- Key facts and context from past conversations
- Active project status
- Decisions and conclusions reached

Do not edit this file manually — it is maintained by the agent and scheduler.

## Chat

### Basic Usage

```bash
# Chat using the default workspace
cobot chat "Explain Go interfaces"

# Chat using a specific workspace
cobot chat -w my-project "What is the status of the auth service?"

# Interactive TUI mode
cobot chat

# Use a specific agent within the workspace
cobot chat -w my-project --agent reviewer "Review my last PR"

# Override model for this session
cobot chat --model anthropic:claude-sonnet-4 "Hello"
```

### TUI Mode

Running `cobot chat` without a message argument launches the interactive TUI. Press `Ctrl+C` or type `/quit` to exit.

### ACP Server Mode

```bash
cobot acp serve
```

Starts an ACP server for IDE or tooling integration. Clients send requests with optional workspace and agent fields:

```json
{
  "workspace": "my-project",
  "agent": "main",
  "message": "hello"
}
```

If `workspace` is omitted, the default workspace is used. If `agent` is omitted, the workspace's `default_agent` is used.

## Memory

Cobot uses a hierarchical memory system called MemPalace. Memory is scoped per workspace.

### MemPalace Structure

- **Wings** — Top-level domains (e.g., "golang", "auth-service", "personal")
- **Rooms** — Contextual spaces within a wing
- **Drawers** — Raw content storage within rooms
- **Closets** — Summarized or aggregated content

### Memory Commands

```bash
# Search workspace memory
cobot memory search "authentication patterns"

# Search within a specific wing
cobot memory search "goroutines" --wing golang

# Show memory structure and statistics
cobot memory status
```

Memory is stored per-workspace in `<data_dir>/<workspace>/memory/` using BadgerDB and a Bleve search index.

## Configuration Reference

### Global Config (`~/.config/cobot/config.yaml`)

```yaml
model: openai:gpt-4o
api_keys:
  openai: ${OPENAI_API_KEY}
  anthropic: ${ANTHROPIC_API_KEY}
max_turns: 50
memory_enabled: true
```

`${VAR_NAME}` patterns are resolved from environment variables at load time.

### Workspace Config (`<data_dir>/<workspace>/workspace.yaml`)

```yaml
id: ws-abc123
name: my-project
type: project
root: /path/to/project
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-04-12T00:00:00Z

enabled_mcp:
  - github
  - filesystem

enabled_skills:
  - code-review
  - debugging

sandbox:
  root: /path/to/project
  allow_paths:
    - /tmp
  readonly_paths: []
  allow_network: true
  blocked_commands:
    - "rm -rf /"

agents:
  main: agents/main.yaml
  reviewer: agents/reviewer.yaml

default_agent: main

summarization:
  enabled: true
  interval: 1h
  include:
    - memory
    - skills
    - soul
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `COBOT_CONFIG_PATH` | Override config directory (default: `~/.config/cobot/`) |
| `COBOT_DATA_PATH` | Override data directory (default: `~/.local/share/cobot/`) |
| `COBOT_WORKSPACE` | Set active workspace by name |
| `COBOT_MODEL` | Override default model for all commands |
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |

Resolution priority for paths: CLI flag (`--config`, `--data`) > environment variable > default.

Example:

```bash
export COBOT_WORKSPACE=my-project
export COBOT_MODEL=anthropic:claude-sonnet-4
export OPENAI_API_KEY=sk-xxx
export ANTHROPIC_API_KEY=sk-xxx

cobot chat "What did we decide about the database schema?"
```

## Sandbox

Each workspace defines sandbox constraints that restrict what agents can access.

### Filesystem Sandbox

The `filesystem_read` and `filesystem_write` tools enforce path restrictions:

- **Allowed paths**: `sandbox.root` + `sandbox.allow_paths` + workspace data directory
- **Readonly paths**: `sandbox.readonly_paths` — readable but not writable
- **Symlink traversal**: Resolved and validated against allowed paths
- **Directory traversal**: `..` sequences are resolved and blocked if they escape allowed scope

### Shell Sandbox

The `shell_exec` tool enforces:

- **Working directory**: Forced to `sandbox.root`
- **Command blocklist**: Checked against `sandbox.blocked_commands` (substring match)
- **Network access**: If `sandbox.allow_network` is false, network-related commands are rejected

### Per-Agent Sandbox Overrides

An agent can narrow the workspace sandbox with its own `sandbox` field. An empty `sandbox: {}` means the agent inherits the workspace sandbox. Non-empty fields override the corresponding workspace settings.

```yaml
# Agent with tighter sandbox
name: untrusted-agent
sandbox:
  root: /path/to/project/src    # More restricted than workspace root
  allow_network: false
```

## CLI Command Reference

```bash
# Setup and health
cobot setup                            # Initialize config, data dir, and default workspace
cobot doctor                           # Check configuration health

# Configuration
cobot config show                      # Show current config
cobot config set <key> <value>         # Set a config value

# Workspace management
cobot workspace list                   # List all workspaces
cobot workspace create <name>          # Create a new workspace
cobot workspace delete <name>          # Delete a workspace
cobot workspace show [name]            # Show workspace config

# Agent management (workspace-scoped)
cobot agent list                       # List agents in active workspace
cobot agent show <name>                # Show agent config

# MCP server management (global registry)
cobot mcp list                         # List registered MCP servers
cobot mcp add <name> -f <file>         # Register MCP server from file
cobot mcp remove <name>                # Unregister MCP server
cobot mcp show <name>                  # Show MCP server definition
cobot mcp test <name>                  # Test MCP server connectivity

# Skills management
cobot skill list                       # List global skills
cobot skill list --workspace           # List workspace-private skills
cobot skill add <name> -f <file>       # Add skill to global registry
cobot skill remove <name>              # Remove skill from global registry
cobot skill show <name>                # Show skill definition

# Chat
cobot chat [message]                   # Chat (default workspace, default agent)
cobot chat -w <workspace> [message]    # Chat with specific workspace
cobot acp serve                        # Start ACP server

# Memory
cobot memory search <query>            # Search workspace memory
cobot memory search <query> --wing <wing>  # Search within a specific wing
cobot memory status                    # Show memory statistics
```

## Troubleshooting

### "No API key configured"

Configure an API key through the CLI or environment:

```bash
cobot config set api_keys.openai sk-your-key-here
# or
export OPENAI_API_KEY=sk-your-key-here
```

### "Workspace not found: my-project"

The workspace has not been registered. Create it first:

```bash
cobot workspace create my-project
```

Or, if you are inside a project directory with a `.cobot/` folder, verify that `workspace.yaml` inside it contains the correct `name` field and that the workspace was created with `cobot workspace create`.

### "Failed to open memory store"

Run setup to ensure required directories are initialized:

```bash
cobot setup
cobot doctor
```

Check that `~/.local/share/cobot/<workspace>/memory/` exists and is writable.

### "Command not found: cobot"

Ensure `$GOPATH/bin` is in your PATH:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

Or install system-wide:

```bash
make install-system
```

### MCP server fails to connect

Test the server directly:

```bash
cobot mcp test github
```

Check that the required environment variables (e.g., `GITHUB_PERSONAL_TOKEN`) are set, and that the `command` or `url` in the MCP definition is correct.

### Agent uses wrong workspace

Workspace is determined at runtime with this priority: CLI flag `-w` > `COBOT_WORKSPACE` env var > `.cobot/workspace.yaml` in the directory tree > `default`. If you are getting the wrong workspace, check whether a `.cobot/workspace.yaml` exists in a parent directory and whether `COBOT_WORKSPACE` is set in your shell.

## Next Steps

- Create a project workspace: `cobot workspace create my-project`
- Add an MCP server: `cobot mcp add github -f github.yaml`
- Browse available agents: `cobot agent list`
- Search workspace memory: `cobot memory search "recent decisions"`
- Read the [Architecture Guide](architecture.md) for implementation details
