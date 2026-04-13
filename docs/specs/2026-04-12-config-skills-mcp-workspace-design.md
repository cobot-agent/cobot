# Config, Skills, MCP & Workspace Architecture Redesign

Date: 2026-04-12
Status: Draft

## Overview

Complete restructuring of cobot's configuration, skills, MCP, and workspace management system. No backward compatibility required.

## Core Principles

1. **Two-tier mutability**: `<config_dir>` is system-level and agent-immutable; `<data_dir>` is workspace-level and agent-mutable in real-time
2. **Registry + Enable pattern**: Global registries for MCP servers and Skills; workspaces selectively enable what they need
3. **Full sandbox**: Agents are restricted to their workspace boundaries (filesystem, shell, tools)
4. **Multi-agent per workspace**: Each workspace can define multiple agents with different models, prompts, and tool sets
5. **Workspace self-evolution**: Workspaces maintain their own SOUL.md, USER.md, skills, and can periodically summarize content

## Path Resolution

All base paths are configurable and resolved in priority order:

| Path | Default | Env Var Override | CLI Flag |
|------|---------|------------------|----------|
| Config dir | `~/.config/cobot/` | `COBOT_CONFIG_PATH` | `--config <dir>` |
| Data dir | `~/.local/share/cobot/` | `COBOT_DATA_PATH` | `--data <dir>` |

Resolution priority: CLI flag > env var > default.

When a custom config/data dir is set, all internal paths derive from it:
- MCP registry: `<config_dir>/mcp/`
- Skills registry: `<config_dir>/skills/`
- Workspace definitions: `<config_dir>/workspaces/`
- Workspace data: `<data_dir>/<workspace-name>/`

## Directory Structure

Using default paths as example:

```
<config_dir>/                            # [System-level] Agent-immutable, managed by CLI
├── config.yaml                           # Global settings (API keys, default model)
├── mcp/                                  # MCP global registry
│   ├── <name>.yaml                       # One file per MCP server
│   └── ...
├── skills/                               # Skills global registry
│   ├── <name>.yaml                       # Single-file YAML skill
│   ├── <name>.md                         # Single-file Markdown skill
│   ├── <name>/                           # Directory-form skill
│   │   ├── SKILL.md
│   │   └── scripts/                      # Optional auxiliary files
│   └── ...
└── workspaces/                           # Workspace definitions (path mappings)
    ├── <name>.yaml                       # Maps workspace name to data directory
    └── ...

<data_dir>/                               # [Workspace-level] Agent-mutable
├── <workspace-name>/                     # One directory per workspace
│   ├── workspace.yaml                    # Workspace config (enabled MCP/skills, sandbox, agents)
│   ├── SOUL.md                           # Agent personality (agent-mutable)
│   ├── USER.md                           # User profile (agent-mutable)
│   ├── MEMORY.md                         # Consolidated memory (agent-mutable)
│   ├── agents/                           # Agent definitions
│   │   ├── <agent-name>.yaml             # Per-agent config (model, tools, skills)
│   │   └── ...
│   ├── skills/                           # Workspace-private skills
│   │   ├── <name>.yaml                   # YAML skill
│   │   ├── <name>.md                     # Markdown skill
│   │   ├── <name>/                       # Directory skill
│   │   │   └── SKILL.md
│   │   └── ...
│   ├── memory/                           # Memory storage (Badger + Bleve)
│   ├── sessions/                         # Chat session data
│   └── scheduler/                        # Scheduled tasks (periodic summarization)
└── ...
```

### Project Discovery

Project workspaces also have a `.cobot/` directory in the project root:

```
<project-root>/.cobot/
├── workspace.yaml                        # Points to workspace name
└── AGENTS.md                             # Project-level agent instructions
```

## Configuration File Formats

### Global Config (`<config_dir>/config.yaml`)

```yaml
model: openai:gpt-4o
api_keys:
  openai: ${OPENAI_API_KEY}
  anthropic: ${ANTHROPIC_API_KEY}
max_turns: 50
memory_enabled: true
```

Environment variable expansion: `${VAR_NAME}` patterns are resolved at load time.

### MCP Registry (`<config_dir>/mcp/<name>.yaml`)

Each file defines one MCP server:

```yaml
name: github
description: GitHub API via MCP
transport: command                    # "command" | "http"
command: npx
args:
  - "@modelcontextprotocol/server-github"
env:
  GITHUB_TOKEN: ${GITHUB_PERSONAL_TOKEN}
```

HTTP transport variant:

```yaml
name: remote-api
description: Remote API server
transport: http
url: http://localhost:8080
headers:
  Authorization: Bearer ${API_TOKEN}
```

### Skills Registry (`<config_dir>/skills/`)

Three supported forms:

**Single-file YAML** (`<name>.yaml`):
```yaml
name: code-review
description: Review code changes
trigger: review
steps:
  - prompt: "Review the following code changes:\n{{input}}"
    tool: filesystem_read
    output: review_result
```

**Single-file Markdown** (`<name>.md`):
```markdown
---
name: debugging
description: Systematic debugging approach
trigger: debug
---

# Debugging Skill

When encountering a bug or error:
1. Reproduce the issue
2. Isolate the problem
3. Identify root cause
4. Propose fix
```

Markdown skills support YAML frontmatter for metadata.

**Directory form** (`<name>/`):
```
<name>/
├── SKILL.md              # Main file with frontmatter + content
└── scripts/              # Optional auxiliary scripts/files
```

### Workspace Definition (`<config_dir>/workspaces/<name>.yaml`)

Maps a workspace name to its data directory. The `path` can point to any location on the filesystem — it defaults to `<data_dir>/<name>` if omitted, but users can specify custom paths (e.g., on a different drive, network mount, or project-specific location).

```yaml
name: my-project
path: ~/.local/share/cobot/my-project    # Defaults to <data_dir>/<name> if omitted
type: project                            # "default" | "project" | "custom"
root: /path/to/project                   # Required for project type (the actual project source directory)
```

Custom path example:

```yaml
name: large-project
path: /Volumes/storage/cobot-workspaces/large-project
type: custom
```

A special `default.yaml` always exists and cannot be deleted:

```yaml
name: default
type: default
# path omitted → defaults to <data_dir>/default
```

### Workspace Config (`<data_dir>/<ws>/workspace.yaml`)

Agent-mutable. The core workspace runtime configuration:

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
  interval: 1h                        # How often to summarize
  include:
    - memory
    - skills
    - soul
```

### Agent Config (`<data_dir>/<ws>/agents/<name>.yaml`)

Agent-mutable. Each agent in the workspace:

```yaml
name: main
model: openai:gpt-4o
system_prompt: SOUL.md                # Reference to .md file or inline text
enabled_mcp:
  - github
  - filesystem
enabled_skills:
  - code-review
  - debugging
max_turns: 50
sandbox: {}                           # Empty = inherit workspace sandbox

# Optional overrides:
# sandbox:
#   root: /path/to/subset
#   allow_paths: []
```

Multi-agent example — a specialized reviewer:

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

## Workspace Selection

No persistent "current workspace" tracking. Workspace is determined at runtime:

| Method | Example |
|--------|---------|
| CLI flag | `cobot chat -w my-project "hello"` |
| Environment variable | `COBOT_WORKSPACE=my-project` |
| Project discovery | Walk up from CWD, find `.cobot/workspace.yaml` |
| ACP/API | `session/new { workspace: "my-project" }` |
| Default | `default` workspace if nothing specified |

Priority: CLI flag > env var > project discovery > default.

## Skills Loading

Skills are loaded in this order (later overrides earlier):

1. **Global registry** — `<config_dir>/skills/` scanned, filtered by workspace's `enabled_skills`
2. **Workspace private** — `<data_dir>/<ws>/skills/` scanned, all loaded
3. **Override** — Same-name workspace skill overrides global skill

Each skill is resolved to a unified internal representation regardless of format (YAML/Markdown/Directory).

## MCP Connection Lifecycle

1. On cobot startup, read workspace's `enabled_mcp` list
2. For each name, load definition from `<config_dir>/mcp/<name>.yaml`
3. `MCPManager.Connect()` to each enabled server
4. Register discovered tools in the tool registry (prefixed: `mcp_<server>_<tool>`)
5. On shutdown, `MCPManager.Close()` all connections

MCP servers are only defined globally. Workspaces cannot define private MCP servers.

## Sandbox Mechanism

### Filesystem Sandbox

`filesystem_read` and `filesystem_write` tools enforce path restrictions:

- **Allowed base paths**: `sandbox.root` + `sandbox.allow_paths` + workspace `DataDir`/`ConfigDir`
- **Readonly paths**: `sandbox.readonly_paths` — allow read, block write
- **Path resolution**: Resolve symlinks, prevent `..` traversal
- **Error**: Return clear error when path is outside allowed scope

### Shell Sandbox

`shell_exec` tool enforces:

- **Working directory**: Forced to `sandbox.root`
- **Command blocklist**: Check against `sandbox.blocked_commands` (substring match)
- **Network**: If `sandbox.allow_network` is false, reject network-related commands

### Tool Availability

- Only register tools from `enabled_mcp` MCP servers
- Only load `enabled_skills` skills from global registry
- Workspace-private skills always available
- Built-in tools always available but respect sandbox constraints

### Per-Agent Sandbox

Agents can override workspace sandbox settings. Empty `sandbox: {}` means inherit workspace settings. Non-empty fields override the corresponding workspace settings.

## Mutability Boundary

| Location | Mutable by Agent | Modified by |
|----------|-----------------|-------------|
| `<config_dir>/config.yaml` | No | `cobot config set` CLI |
| `<config_dir>/mcp/*.yaml` | No | `cobot mcp add/remove` CLI |
| `<config_dir>/skills/**` | No | `cobot skill add/remove` CLI |
| `<config_dir>/workspaces/*.yaml` | No | `cobot workspace create/delete` CLI |
| `<data_dir>/<ws>/workspace.yaml` | **Yes** | Agent tool + CLI |
| `<data_dir>/<ws>/agents/*.yaml` | **Yes** | Agent tool + CLI |
| `<data_dir>/<ws>/skills/**` | **Yes** | Agent tool + CLI |
| `<data_dir>/<ws>/SOUL.md` | **Yes** | Agent tool + CLI |
| `<data_dir>/<ws>/USER.md` | **Yes** | Agent tool + CLI |
| `<data_dir>/<ws>/MEMORY.md` | **Yes** | Agent tool + scheduler |

Agent-mutable files are modified through dedicated tools:
- `workspace_config_update` — modify workspace.yaml
- `agent_config_update` — modify agent configs
- `skill_create` / `skill_update` — manage workspace-private skills
- `persona_update` — modify SOUL.md / USER.md
- Scheduler handles periodic MEMORY.md summarization

## Workspace Self-Evolution

Workspaces maintain evolving state through:

1. **SOUL.md**: Agent personality, updated through interactions
2. **USER.md**: User profile, updated as agent learns about the user
3. **MEMORY.md**: Consolidated memory, periodically summarized by scheduler
4. **Skills**: Agent can create new skills in workspace-private skills directory
5. **Summarization**: Configurable interval, consolidates memory, skills, and persona

## CLI Commands

### MCP Management (system-level)
```
cobot mcp list                          # List registered MCP servers
cobot mcp add <name> -f <file>         # Register from file
cobot mcp remove <name>                # Unregister
cobot mcp show <name>                  # Show server details
cobot mcp test <name>                  # Test connection
```

### Skills Management
```
cobot skill list [--global|--workspace] # List skills
cobot skill add <name> -f <file>       # Add to global registry
cobot skill remove <name>              # Remove from global registry
cobot skill show <name>                # Show skill details
```

### Workspace Management
```
cobot workspace list                   # List workspaces
cobot workspace create <name>          # Create workspace
cobot workspace delete <name>          # Delete workspace
cobot workspace show [name]            # Show workspace config
```

### Agent Management (workspace-level)
```
cobot agent list                       # List agents in current workspace
cobot agent show <name>                # Show agent config
```

## ACP API Changes

`session/new` request extended:

```json
{
  "workspace": "my-project",
  "agent": "main",
  "message": "hello"
}
```

If `workspace` omitted, uses default. If `agent` omitted, uses workspace's `default_agent`.

## Migration from Current Architecture

Since no backward compatibility is required:

1. **Add** path resolution layer: `COBOT_CONFIG_PATH` / `COBOT_DATA_PATH` env vars, `--config` / `--data` CLI flags
2. **Remove** `<config_dir>/workspaces/manager.yaml` (no current workspace tracking)
3. **Move** workspace configs from `<config_dir>/workspaces/<name>/workspace.yaml` to `<config_dir>/workspaces/<name>.yaml` (flat file with path mapping)
4. **Consolidate** workspace data under `<data_dir>/<name>/`
5. **Create** `<config_dir>/mcp/` directory, migrate MCP definitions from config.yaml
6. **Create** `<config_dir>/skills/` directory for global skills
7. **Add** `agents/` subdirectory to each workspace data dir
8. **Rewrite** config loading pipeline to support new structure
9. **Add** sandbox enforcement to filesystem and shell tools
10. **Add** workspace-mutable config tools for agent use
