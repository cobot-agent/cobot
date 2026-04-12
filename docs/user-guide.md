# User Guide - Getting Started with Cobot Personal Agent

## Overview

Cobot is a **personal AI agent** that maintains a single global memory space across all your interactions. Unlike project-based agents, Cobot learns from all your conversations and maintains context regardless of which directory you're working in.

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/cobot-agent/cobot.git
cd cobot

# Build
make build

# Or install to $GOPATH/bin
make install

# Or install system-wide
make install-system
```

### Quick Install (macOS/Linux)

```bash
curl -sSL https://raw.githubusercontent.com/cobot-agent/cobot/main/install.sh | bash
```

## First-Time Setup

### 1. Run Setup Wizard

```bash
cobot setup
```

This interactive wizard will:
- Create `~/.config/cobot/` directory
- Create `~/.local/share/cobot/` directory  
- Generate default `SOUL.md` (bot personality)
- Generate default `USER.md` (your profile)
- Generate default `MEMORY.md` (consolidated memories)
- Save your API key configuration

### 2. Verify Installation

```bash
cobot doctor
```

Expected output:
```
Cobot Personal Agent Doctor
===========================

Config directory: /Users/you/.config/cobot
  [OK] Directory exists
  [OK] Config file: /Users/you/.config/cobot/config.yaml
  [OK] Model: openai:gpt-4o
  [OK] API keys configured: [openai]

Persona files:
  [OK] SOUL:   /Users/you/.config/cobot/SOUL.md
  [OK] USER:   /Users/you/.config/cobot/USER.md
  [OK] MEMORY: /Users/you/.config/cobot/MEMORY.md

Data directory: /Users/you/.local/share/cobot
  [OK] Directory exists
  [OK] Memory dir: /Users/you/.local/share/cobot/memory

All critical checks passed!
```

## Directory Structure

After setup, your system will have:

```
~/.config/cobot/              # Configuration (XDG_CONFIG_HOME)
├── config.yaml               # Main configuration
├── SOUL.md                   # Bot personality & voice
├── USER.md                   # User profile & preferences
└── MEMORY.md                 # Consolidated memories

~/.local/share/cobot/         # Data (XDG_DATA_HOME)
├── memory/                   # Global MemPalace storage
│   ├── badger/               # BadgerDB database
│   └── bleve/                # Search index
├── sessions/                 # Session data
└── skills/                   # Skill storage
```

## Basic Usage

### Chat with Your Agent

```bash
# Interactive TUI mode (works from any directory)
cobot chat

# One-shot command
cobot chat "Explain Go interfaces"

# With specific model
cobot chat --model anthropic:claude-3-opus "Hello!"
```

### Customize Your Agent

```bash
# Edit bot personality
cobot persona edit soul

# Edit your profile  
cobot persona edit user

# View current settings
cobot persona show soul
cobot persona show user
```

### Search Your Memory

```bash
# Search everything you've learned
cobot memory search "authentication patterns"

# View memory structure
cobot memory status

# Filter by domain (wing)
cobot memory search "goroutines" --wing golang
```

### Configuration

```bash
# View configuration
cobot config show

# Set configuration values
cobot config set model openai:gpt-4o
cobot config set max_turns 100

# Set API keys
cobot config set apikey.openai sk-xxx
cobot config set apikey.anthropic sk-xxx
```

## Persona Files

### SOUL.md - Bot Personality

Edit this file to customize how Cobot communicates:

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
- Ask clarifying questions when ambiguous
```

### USER.md - User Profile

Edit this file to tell Cobot about yourself:

```markdown
# USER

## Profile
- Name: Alex
- Role: Senior Software Engineer
- Experience: 8+ years

## Preferences
- Languages: Go, TypeScript, Python
- Editor: VS Code
- Values: Clean code, performance, testing

## Work Style
- Morning person
- Test-driven development
- Prefers practical solutions
```

### MEMORY.md - Consolidated Memories

This file is automatically updated by the consolidation process. It contains:
- Key facts learned from conversations
- Active projects and their status
- Important context to remember

## Advanced Usage

### Environment Variables

```bash
# Override model
export COBOT_MODEL=anthropic:claude-3-opus

# Set API keys
export OPENAI_API_KEY=sk-xxx
export ANTHROPIC_API_KEY=sk-xxx

# Custom config location
export COBOT_CONFIG=/path/to/config.yaml
```

### Working with Wings

Wings are top-level memory domains:

```bash
# Search within a specific wing
cobot memory search "middleware" --wing golang

# Wings are automatically created based on context
# Common wings: "golang", "python", "personal", "work"
```

### ACP Server Mode

```bash
# Start ACP server for IDE integration
cobot acp serve
```

## Troubleshooting

### Issue: "No API key configured"

**Solution:**
```bash
cobot config set apikey.openai sk-your-key-here
# or
export OPENAI_API_KEY=sk-your-key-here
```

### Issue: "Failed to open memory store"

**Solution:**
```bash
# Ensure directories exist
cobot setup

# Check permissions
cobot doctor
```

### Issue: Commands not found after install

**Solution:**
```bash
# Ensure $GOPATH/bin is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Or install system-wide
make install-system
```

## Tips

1. **Run from anywhere**: Unlike project-based agents, Cobot works from any directory
2. **Personalize early**: Edit SOUL.md and USER.md to get better responses
3. **Memory persists**: Your conversations build up knowledge over time
4. **Search is powerful**: Use `cobot memory search` to recall past discussions
5. **Keep it updated**: Run `cobot doctor` periodically to check configuration

## Next Steps

- Explore `cobot memory status` to see your memory structure
- Customize your persona files for better interactions
- Try the TUI mode with `cobot chat` for interactive sessions
- Read the [Architecture Guide](architecture.md) to understand how it works
