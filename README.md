# Cobot - Personal AI Agent

[![Go Version](https://img.shields.io/badge/go-1.26.2-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Cobot is a **personal AI agent** that learns from all your interactions across contexts. Unlike project-based agents, Cobot maintains a single global memory space that persists across all your work.

Inspired by [nanobot](https://github.com/HKUDS/nanobot) and MemPalace architecture.

## 🌟 Features

### Core Capabilities
- **🧠 Global Memory**: Single memory space across all contexts (not per-project)
- **👤 Persona Layer**: SOUL.md (bot personality) + USER.md (user profile)
- **🏛️ MemPalace**: Hierarchical memory (Wings → Rooms → Drawers → Closets)
- **🔧 Tool System**: Built-in tools + MCP integration
- **🤖 Multi-Provider**: OpenAI and Anthropic LLM support
- **💬 Interactive Modes**: CLI, TUI, and ACP server

### Personal Agent Architecture

```
~/.config/cobot/              # Configuration (XDG)
├── config.yaml               # Main configuration
├── SOUL.md                   # Bot personality & voice
├── USER.md                   # User profile & preferences
└── MEMORY.md                 # Consolidated memories

~/.local/share/cobot/         # Data (XDG)
├── memory/                   # Global MemPalace storage
│   ├── badger/               # BadgerDB
│   └── bleve/                # Search index
├── sessions/                 # Session data
└── skills/                   # Skill storage
```

### Memory Hierarchy (MemPalace)
- **Wings**: Domains/Topics (e.g., "golang", "machine-learning")
- **Rooms**: Contextual spaces within wings
- **Drawers**: Raw content storage
- **Closets**: Summarized/aggregated content
- **Knowledge Graph**: Temporal triples with inference

## 🚀 Installation

### Prerequisites
- Go 1.26.2 or later
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

## 📝 Quick Start

### 1. First-Time Setup
```bash
cobot setup
```

This creates:
- `~/.config/cobot/config.yaml` - Your configuration
- `~/.config/cobot/SOUL.md` - Bot personality
- `~/.config/cobot/USER.md` - Your profile
- `~/.local/share/cobot/memory/` - Global memory storage

### 2. Configure API Keys
```bash
# Set your OpenAI API key
cobot config set apikey.openai sk-xxx

# Or set environment variable
export OPENAI_API_KEY=sk-xxx
```

### 3. Customize Your Agent
```bash
# Edit bot personality
cobot persona edit soul

# Edit your profile
cobot persona edit user

# View current settings
cobot persona show soul
cobot persona show user
```

### 4. Start Chatting
```bash
# Interactive TUI mode (works from any directory)
cobot chat

# One-shot command
cobot chat "Explain Go interfaces"

# With specific model
cobot chat --model anthropic:claude-3-opus "Hello!"
```

## 🎭 Persona Files

### SOUL.md - Bot Personality
```markdown
# SOUL

You are Cobot, a personal AI assistant.

## Voice
- Concise and direct
- Technical but accessible
- Use analogies when helpful

## Style
- Prefer code examples over explanations
- Always suggest best practices
```

### USER.md - User Profile
```markdown
# USER

## Profile
- Name: Developer
- Role: Software Engineer
- Experience: 5+ years

## Preferences
- Likes: Go, TypeScript, Python
- Editor: VS Code
- Values: Clean code, performance

## Work Style
- Morning person
- Test-driven development
```

## 🧠 Memory Commands

```bash
# Search your memory
cobot memory search "authentication patterns"

# View memory palace structure
cobot memory status

# Filter by wing
cobot memory search "goroutines" --wing golang
```

## 🧪 Testing

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/memory/...
```

## 🔧 Configuration

### Config File
`~/.config/cobot/config.yaml`:
```yaml
model: openai:gpt-4o
max_turns: 50
temperature: 0.7
apikeys:
  openai: sk-xxx
  anthropic: sk-xxx
memory:
  enabled: true
```

### Environment Variables
```bash
COBOT_MODEL=openai:gpt-4o
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-xxx
```

## 📝 CLI Commands

```bash
# Setup & Configuration
cobot setup                       # First-time setup
cobot doctor                      # Check configuration
cobot config show                 # Show config
cobot config set <key> <value>    # Set config value

# Persona Management
cobot persona init                # Create persona files
cobot persona edit soul           # Edit SOUL.md
cobot persona edit user           # Edit USER.md
cobot persona show memory         # Show MEMORY.md

# Chat & Interaction
cobot chat [message]              # Chat with agent
cobot tui                         # Interactive TUI mode
cobot acp serve                   # Start ACP server

# Memory
cobot memory search <query>       # Search memory
cobot memory status               # Show memory stats

# Tools & Models
cobot tools list                  # List available tools
cobot model list                  # List available models

# Workspace (legacy - not needed for personal agent)
cobot workspace init              # Create project workspace
```

## 🏗️ Architecture

```
cobot/
├── cmd/cobot/            # CLI commands
│   ├── chat.go
│   ├── persona_cmd.go    # Persona management
│   ├── config_cmd.go
│   └── ...
├── internal/
│   ├── agent/            # Core agent loop
│   ├── memory/           # MemPalace storage
│   ├── persona/          # SOUL.md, USER.md management
│   ├── llm/              # LLM providers
│   └── workspace/        # Workspace utilities
└── pkg/                  # Public SDK
```

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Write tests
4. Ensure tests pass: `go test ./...`
5. Commit: `git commit -m 'feat: add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open Pull Request

## 📄 License

MIT License - see [LICENSE](LICENSE) file

## 🙏 Acknowledgments

- Inspired by [nanobot](https://github.com/HKUDS/nanobot) personal agent concept
- MemPalace architecture for hierarchical memory
- Uses [BadgerDB](https://github.com/dgraph-io/badger) for storage
- Uses [Bleve](https://github.com/blevesearch/bleve) for search
