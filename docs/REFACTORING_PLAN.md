# Cobot → Personal Agent Refactoring Plan

## Overview

Transform cobot from **project-based** agent (per-project workspace with hashed memory) to **personal agent** (single global memory space like nanobot).

## Current State (Project-Based)

```
~/my-project/
├── .cobot/
│   ├── config.yaml
│   └── AGENTS.md
└── ... (project files)

~/.local/share/cobot/
└── workspaces/
    └── <sha256-hash-of-project-root>/
        └── memory/     # Isolated per-project memory
            ├── badger/
            └── bleve/
```

**Problems**:
- Memory isolated per project - no cross-project learning
- Must be in project directory to access context
- Multiple memory stores for same user
- Complex workspace discovery logic

## Target State (Personal Agent - XDG Compliant)

```
~/.config/cobot/             # Config (XDG_CONFIG_HOME)
├── config.yaml              # Main configuration
├── SOUL.md                  # Bot's personality/voice
├── USER.md                  # User profile & preferences
├── TOOLS.md                 # Tool usage patterns
└── AGENTS.md                # Sub-agent definitions

~/.local/share/cobot/        # Data (XDG_DATA_HOME)
├── memory/                  # Single global memory store (MemPalace)
│   ├── badger/              # BadgerDB storage
│   └── bleve/               # Bleve search index
├── history.jsonl            # Structured conversation history
└── wakelog.md               # Wake/consolidation log
```

**Benefits**:
- Persistent personal assistant across all contexts
- Unified memory - learns from all interactions
- Simpler architecture - no project discovery
- Cross-project insights and patterns

## Architecture Changes

### Memory Model Comparison

| Aspect | Current (Cobot) | Target (Personal Agent) |
|--------|----------------|----------------------|
| **Scope** | Per-project | Global personal |
| **Storage** | Hashed workspace dirs | Single `~/.cobot/` |
| **Memory Architecture** | **MemPalace preserved** | Wings→Rooms→Drawers→Closets |
| **Context Building** | WakeUp (L0-L3 layers) | WakeUp + Persona files |
| **Consolidation** | Manual | Token budget-driven |
| **New Components** | None | SOUL.md, USER.md (persona layer) |

### Key Design Decisions

1. **Keep Wings/Rooms?** 
   - Option A: Remove entirely - use only nanobot-style files
   - Option B: Map Wings to "domains" in MEMORY.md sections
   - **Decision**: Option B - Wings become sections in MEMORY.md

2. **Migration Strategy**
   - Phase 1: Support both modes (backward compatible)
   - Phase 2: Migrate existing workspaces
   - Phase 3: Remove legacy code

## Phase 1: Global Memory Directory

### Changes

1. **Remove workspace hashing**
   - File: `internal/workspace/workspace.go`
   - Remove: `workspaceDataDir(projectRoot string)` function
   - Replace with: `GlobalMemoryDir() string`

2. **Update discovery**
   - File: `internal/workspace/discovery.go`
   - Make workspace discovery optional
   - Default to `~/.cobot/` when not in project

3. **Update CLI helpers**
   - File: `cmd/cobot/helpers.go`
   - Change memory store initialization to use global path

### Code Example (XDG Compliant)

```go
// Before (workspace.go) - Project-based with hashing
func workspaceDataDir(projectRoot string) string {
    hash := sha256.Sum256([]byte(projectRoot))
    short := hex.EncodeToString(hash[:])[:16]
    return filepath.Join(xdg.DataHome(), "cobot", "workspaces", short)
}

// After (persona.go - new file) - Personal agent
func ConfigDir() string {
    return filepath.Join(xdg.ConfigHome(), "cobot")  // ~/.config/cobot
}

func DataDir() string {
    return filepath.Join(xdg.DataHome(), "cobot")    // ~/.local/share/cobot
}

func MemoryDir() string {
    return filepath.Join(DataDir(), "memory")
}

func HistoryFile() string {
    return filepath.Join(DataDir(), "history.jsonl")
}

func SoulFile() string {
    return filepath.Join(ConfigDir(), "SOUL.md")
}

func UserFile() string {
    return filepath.Join(ConfigDir(), "USER.md")
}
```

## Phase 2: Persona Layer (Add-on to MemPalace)

**MemPalace 架构完全保留**: Wings → Rooms → Drawers → Closets

新增 **Persona 层**（nanobot 风格），与 MemPalace 互补：

### New Files to Create

1. **`internal/persona/soul.go`**
   - Manages SOUL.md (bot personality)
   - Bot's voice, style, communication preferences
   - Loaded into system prompt

2. **`internal/persona/user.go`**
   - Manages USER.md (user profile)
   - User preferences, work style, relationships
   - Stable knowledge about the user

3. **`internal/persona/consolidator.go`**
   - Token budget-driven memory consolidation
   - Updates MemPalace Closets automatically
   - Extracts facts from conversations

### Architecture

```
Persona Layer (NEW)
├── SOUL.md              → Bot personality → System prompt
├── USER.md              → User profile → System prompt
└── Consolidator         → Auto-summarize → MemPalace Closets

MemPalace Layer (PRESERVED)
├── Wings                → Domains/Topics (not projects!)
│   ├── Room: facts      → Closets (summaries)
│   ├── Room: log        → Drawers (raw content)
│   └── Room: code       → Knowledge Graph
```

### Wing = Domain (not Project)

**Before**: Wing = Project (my-project/)
**After**: Wing = Domain/Topic (e.g., "golang", "machine-learning", "personal")

Wings are user-defined organizational units, not tied to filesystem paths.

### SOUL.md Example

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

### USER.md Example

```markdown
# USER

## Profile
- Name: Developer
- Role: Software Engineer
- Experience: 5+ years

## Preferences
- Dark mode enthusiast
- Likes TypeScript, Go, Python
- Prefers shell scripts for automation
- Values performance optimization

## Work Style
- Morning person
- Uses VS Code
- Git power user
- Test-driven development
```

## Phase 3: Dual-Layer Memory Architecture

### Layer 1: MEMORY.md (Prompt Context)

- **Always loaded** into system prompt
- **Full replacement** on consolidation
- **Token budget aware**
- Contains: User info, active projects, important notes

### Layer 2: History (Searchable Archive)

- **Append-only** - never modified
- **Search interface** - grep-style queries
- **Not loaded** into prompt directly
- Contains: Raw conversation history, events

### Consolidation Pipeline

```go
func (p *Persona) Consolidate(ctx context.Context, session *Session) error {
    // 1. Check token budget
    if p.tokenCount < p.budgetThreshold {
        return nil // No need to consolidate
    }
    
    // 2. Summarize oldest messages
    summary, err := p.llm.Summarize(ctx, session.OldestMessages())
    if err != nil {
        return err // Fallback to RAW archive
    }
    
    // 3. Update MEMORY.md surgically
    return p.updateMemoryMD(summary)
}
```

## Phase 4: CLI & Config Updates

### New Commands

```bash
# Initialize personal workspace
cobot init --personal

# Edit core files
cobot edit soul      # Edit SOUL.md
cobot edit user      # Edit USER.md
cobot edit memory    # Edit MEMORY.md

# View history
cobot history        # Show recent history
cobot history search "auth service"  # Search history

# Force consolidation
cobot consolidate    # Run memory consolidation now
```

### Config Changes

```yaml
# ~/.cobot/config.yaml
model: openai:gpt-4o
memory:
  mode: personal  # vs project
  consolidation:
    enabled: true
    budget_threshold: 0.5  # 50% of context window
    dream_interval: 1h     # Background consolidation
persona:
  soul_file: SOUL.md
  user_file: USER.md
  memory_file: MEMORY.md
```

## Phase 5: Migration & Testing

### Migration Path

1. **Detect existing workspaces**
   ```bash
   find ~ -name ".cobot" -type d 2>/dev/null
   ```

2. **Migrate data**
   - Copy Wings/Rooms/Drawers to new structure
   - Generate initial MEMORY.md from existing data
   - Keep backup of old structure

3. **Testing Strategy**
   - Unit tests for new persona package
   - Integration tests for consolidation
   - E2E tests for CLI commands
   - Migration tests for existing workspaces

## Files to Modify

| Phase | File | Change |
|-------|------|--------|
| 1 | `internal/workspace/workspace.go` | Remove hashing, add global paths |
| 1 | `internal/workspace/discovery.go` | Make optional |
| 1 | `cmd/cobot/helpers.go` | Use global memory |
| 2 | `internal/persona/soul.go` | **NEW** - SOUL.md management |
| 2 | `internal/persona/user.go` | **NEW** - USER.md management |
| 2 | `internal/persona/memory.go` | **NEW** - MEMORY.md management |
| 2 | `internal/persona/history.go` | **NEW** - History management |
| 3 | `internal/memory/consolidator.go` | **NEW** - Token budget consolidation |
| 4 | `cmd/cobot/init.go` | Add --personal flag |
| 4 | `cmd/cobot/persona.go` | **NEW** - Persona editing commands |
| 5 | All test files | Update for new structure |

## Success Metrics

- [ ] Single `~/.cobot/` directory created on init
- [ ] SOUL.md, USER.md, MEMORY.md auto-created
- [ ] Conversations persist across different directories
- [ ] Token budget consolidation works
- [ ] Can search history across all sessions
- [ ] Existing tests pass (backward compatibility)
- [ ] Migration script works for existing workspaces

## References

- Nanobot: https://github.com/HKUDS/nanobot
- Cobot Original Design: `docs/specs/2026-04-12-cobot-agent-system-design.md`
