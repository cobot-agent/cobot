# Cobot

[![Go Version](https://img.shields.io/badge/go-1.26-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Cobot is a multi-workspace AI agent framework with isolated per-workspace memory, persona, and skill sets. Each workspace maintains its own agents, sandbox configuration, and evolving state — enabling project-specific AI workflows that are fully isolated from one another.

## Features

- **Multi-workspace**: Each workspace has isolated memory, persona files, skills, agents, and sessions
- **Two-tier mutability**: System-level config (`~/.config/cobot/`) is agent-immutable; workspace data (`~/.local/share/cobot/`) is agent-mutable at runtime
- **Registry + Enable pattern**: Global registries for MCP servers and Skills; workspaces selectively enable what they need
- **Multi-agent per workspace**: Each workspace defines multiple agents with different models, prompts, and tool sets
- **Workspace self-evolution**: Agents can update SOUL.md, USER.md, MEMORY.md, and create private skills within their workspace
- **MemPalace memory**: Hierarchical memory storage (Wings -> Rooms -> Drawers -> Closets) backed by BadgerDB and Bleve full-text search
- **Sandbox enforcement**: Per-workspace and per-agent filesystem and shell sandboxing
- **Project discovery**: Place a `.cobot/` directory in any project root to bind it to a workspace
- **Multi-provider LLM**: OpenAI and Anthropic support

## Directory Structure

```
~/.config/cobot/                         # System-level, agent-immutable
├── config.yaml                          # Global settings (API keys, default model)
├── mcp/                                 # MCP global registry
│   └── <name>.yaml                      # One file per MCP server
├── skills/                              # Skills global registry
│   ├── <name>.yaml                      # Single-file YAML skill
│   ├── <name>.md                        # Single-file Markdown skill
│   └── <name>/                          # Directory-form skill
│       ├── SKILL.md
│       └── scripts/
└── workspaces/                          # Workspace definitions (path mappings)
    └── <name>.yaml                      # Maps workspace name to data directory

~/.local/share/cobot/                    # Workspace-level, agent-mutable
└── <workspace-name>/
    ├── workspace.yaml                   # Workspace config (enabled MCP/skills, sandbox, agents)
    ├── SOUL.md                          # Agent personality (agent-mutable)
    ├── USER.md                          # User profile (agent-mutable)
    ├── MEMORY.md                        # Consolidated memory (agent-mutable)
    ├── agents/
    │   └── <agent-name>.yaml           # Per-agent config (model, tools, skills)
    ├── skills/                          # Workspace-private skills
    ├── memory/                          # BadgerDB + Bleve indexes
    ├── sessions/                        # Chat session data
    └── scheduler/                       # Periodic summarization tasks
```

### Project Discovery

Add a `.cobot/` directory to any project root to bind it to a workspace:

```
<project-root>/.cobot/
├── workspace.yaml      # Points to workspace name
└── AGENTS.md           # Project-level agent instructions
```

When running `cobot` from inside a project, it automatically detects the workspace via this file.

## Installation

### Prerequisites

- Go 1.26 or later
- Git

### From Source

```bash
git clone https://github.com/cobot-agent/cobot.git
cd cobot
go build -o cobot ./cmd/cobot
```

### Install to $GOPATH/bin

```bash
go install github.com/cobot-agent/cobot/cmd/cobot@latest
```

## Quick Start

### 1. Configure API Keys

```bash
cobot config set api_keys.openai sk-xxx

# Or via environment variables
export OPENAI_API_KEY=sk-xxx
export ANTHROPIC_API_KEY=sk-xxx
```

### 2. Create a Workspace

```bash
cobot workspace create my-project
```

### 3. Register MCP Servers (optional)

```bash
cobot mcp add github -f github.yaml
```

### 4. Start Chatting

```bash
# Use the default workspace
cobot chat "Explain Go interfaces"

# Target a specific workspace
cobot chat -w my-project "Review recent changes"

# Workspace auto-detected from project directory
cd /path/to/my-project
cobot chat "What tests are failing?"
```

## Workspace Selection

Workspace is resolved at runtime — there is no persistent "current workspace" state:

| Method | Example |
|--------|---------|
| CLI flag | `cobot chat -w my-project "hello"` |
| Environment variable | `COBOT_WORKSPACE=my-project cobot chat "hello"` |
| Project discovery | Walk up from CWD, find `.cobot/workspace.yaml` |
| Default | `default` workspace if nothing else matches |

Priority: CLI flag > environment variable > project discovery > default.

## CLI Reference

### MCP Management

```bash
cobot mcp list                    # List registered MCP servers
cobot mcp add <name> -f <file>   # Register from file
cobot mcp remove <name>           # Unregister
cobot mcp show <name>             # Show server details
cobot mcp test <name>             # Test connection
```

### Skills Management

```bash
cobot skill list [--global|--workspace]  # List skills
cobot skill add <name> -f <file>        # Add to global registry
cobot skill remove <name>               # Remove from global registry
cobot skill show <name>                 # Show skill details
```

### Workspace Management

```bash
cobot workspace list              # List workspaces
cobot workspace create <name>     # Create workspace
cobot workspace delete <name>     # Delete workspace
cobot workspace show [name]       # Show workspace config
```

### Agent Management

```bash
cobot agent list                  # List agents in current workspace
cobot agent show <name>           # Show agent config
```

### Chat

```bash
cobot chat "message"              # Chat using resolved workspace
cobot chat -w my-project "message"  # Explicit workspace
```

### Persona

```bash
cobot persona edit soul           # Edit SOUL.md
cobot persona edit user           # Edit USER.md
cobot persona show soul           # Print SOUL.md
cobot persona show user           # Print USER.md
```

### Memory

```bash
cobot memory search "query"       # Full-text search across MemPalace
cobot memory status               # Show memory storage stats
```

### Configuration

```bash
cobot config show                 # Print resolved config
cobot config set <key> <value>    # Set a config value
```

## Configuration

### Global Config (`~/.config/cobot/config.yaml`)

```yaml
model: openai:gpt-4o
api_keys:
  openai: ${OPENAI_API_KEY}
  anthropic: ${ANTHROPIC_API_KEY}
max_turns: 50
memory_enabled: true
```

Environment variable expansion: `${VAR_NAME}` patterns are resolved at load time.

### MCP Server (`~/.config/cobot/mcp/<name>.yaml`)

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

### Workspace Config (`~/.local/share/cobot/<name>/workspace.yaml`)

```yaml
name: my-project
type: project
root: /path/to/project

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
  allow_network: true

agents:
  main: agents/main.yaml
  reviewer: agents/reviewer.yaml

default_agent: main
```

### Agent Config (`~/.local/share/cobot/<name>/agents/<agent>.yaml`)

```yaml
name: main
model: openai:gpt-4o
system_prompt: SOUL.md
enabled_mcp:
  - github
  - filesystem
enabled_skills:
  - code-review
max_turns: 50
sandbox: {}    # Empty = inherit workspace sandbox
```

## Memory Architecture (MemPalace)

Each workspace maintains an independent MemPalace backed by BadgerDB (key-value store) and Bleve (full-text search):

- **Wings**: Top-level domains (e.g., `golang`, `architecture`)
- **Rooms**: Contextual spaces within a wing
- **Drawers**: Raw content entries within a room
- **Closets**: Summarized or aggregated content

Periodic summarization (configurable interval) consolidates memory, skills, and persona files into MEMORY.md.

## Sandbox Enforcement

### Filesystem

`filesystem_read` and `filesystem_write` tools enforce:

- Allowed paths: `sandbox.root` + `sandbox.allow_paths` + workspace data directory
- Readonly paths: allow read, block write
- Symlink resolution and `..` traversal prevention

### Shell

`shell_exec` tool enforces:

- Working directory forced to `sandbox.root`
- Command blocklist checked by substring match
- Network commands blocked if `sandbox.allow_network` is false

### Per-Agent Override

Agents inherit their workspace's sandbox by default. Non-empty `sandbox` fields in an agent config override the corresponding workspace settings.

## Workspace Self-Evolution

Agents can update their own workspace state through dedicated tools:

| File | Tool |
|------|------|
| `SOUL.md`, `USER.md` | `persona_update` |
| `MEMORY.md` | `persona_update` + scheduler |
| `workspace.yaml` | `workspace_config_update` |
| `agents/<name>.yaml` | `agent_config_update` |
| `skills/` | `skill_create`, `skill_update` |

Config-dir files (`~/.config/cobot/`) are never modified by agents — only by CLI commands.

## Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/memory/...
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Write tests for new behavior
4. Ensure all tests pass: `go test ./...`
5. Commit: `git commit -m 'feat: add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request

## License

MIT License — see [LICENSE](LICENSE) for details.

## Acknowledgments

- [BadgerDB](https://github.com/dgraph-io/badger) — embedded key-value store for MemPalace
- [Bleve](https://github.com/blevesearch/bleve) — full-text search for memory queries
