# Cobot Agent System — Architecture Design

**Date:** 2026-04-12
**Status:** Draft
**Author:** Cobot Team

## Overview

Cobot is a Go-based personal agent system providing a CLI tool and an importable SDK (`pkg/`). It implements a complete agent loop with LLM provider integration, structured memory (MemPalace-inspired), MCP client/server, ACP server, subagent coordination, workspace management, and a scheduler.

**Reference projects:**
- Core agent: [Hermes Agent](https://github.com/NousResearch/hermes-agent) — agent loop, tools, skills, cron, messaging
- Memory system: [MemPalace](https://github.com/milla-jovovich/mempalace) — wing/room/closet/drawer structure, L0-L3 memory stack
- MCP: [go-sdk](https://github.com/modelcontextprotocol/go-sdk) — official Go MCP SDK
- ACP: [Agent Client Protocol](https://agentclientprotocol.com) — JSON-RPC 2.0 over stdio

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go 1.26+ | Performance, single binary, good concurrency |
| LLM providers | Plugin architecture, OpenAI-first | Extensible, start practical |
| Memory storage | BadgerDB + Bleve | Embedded, zero external deps, full-text + vector |
| ACP transport | JSON-RPC 2.0 over stdio | Per ACP spec |
| MCP | Official go-sdk | Standard compliance, maintained by MCP team + Google |
| CLI framework | cobra + bubbletea | Industry standard, rich TUI |
| SDK shape | Go package library (`pkg/`) | Idiomatic Go, cobra-desktop imports directly |
| Workspace | Directory-scoped `.cobot/` | Isolation, project-specific config |

---

## 1. Project Structure

```
cobot/
├── go.mod
├── cmd/
│   └── cobot/                  # CLI entry point
│       └── main.go
├── internal/                   # Private implementation
│   ├── agent/                  # Core agent loop
│   │   ├── agent.go            # Agent struct, main loop
│   │   ├── loop.go             # Think → Act → Observe cycle
│   │   ├── context.go          # Context loading (system prompt, memory, history)
│   │   └── session.go          # Session management
│   ├── llm/                    # LLM provider plugin system
│   │   ├── provider.go         # Provider interface
│   │   ├── registry.go         # Provider registry
│   │   ├── openai/             # OpenAI-compatible (covers OpenRouter, local)
│   │   │   ├── provider.go
│   │   │   ├── stream.go
│   │   │   └── types.go
│   │   ├── anthropic/          # Anthropic provider (future)
│   │   └── types.go            # Shared types (Message, ToolCall, etc.)
│   ├── memory/                 # Memory subsystem
│   │   ├── store.go            # MemoryStore interface
│   │   ├── badger/             # BadgerDB KV store
│   │   │   ├── store.go
│   │   │   ├── drawers.go      # Raw verbatim content
│   │   │   ├── closets.go      # Summaries
│   │   │   └── metadata.go     # Wing/room metadata
│   │   ├── search/             # Bleve search engine
│   │   │   ├── index.go        # Full-text + vector index
│   │   │   └── query.go        # Search with wing/room filters
│   │   ├── palace/             # Palace structure
│   │   │   ├── wing.go
│   │   │   ├── room.go
│   │   │   ├── closet.go
│   │   │   ├── drawer.go
│   │   │   └── tunnel.go       # Cross-wing connections
│   │   ├── knowledge/          # Knowledge graph (temporal triples)
│   │   │   ├── graph.go
│   │   │   └── triple.go
│   │   ├── layers/             # L0-L3 memory stack
│   │   │   └── stack.go
│   │   └── miner/              # Data mining (files, conversations)
│   │       ├── file_miner.go
│   │       └── convo_miner.go
│   ├── tools/                  # Tool execution framework
│   │   ├── registry.go         # Tool registry
│   │   ├── executor.go         # Parallel tool execution
│   │   ├── builtin/            # Built-in tools
│   │   │   ├── filesystem.go   # File read/write/search
│   │   │   ├── shell.go        # Shell command execution
│   │   │   ├── web.go          # Web fetch/search
│   │   │   └── memory.go       # Memory search/store tools
│   │   └── mcp/                # MCP integration
│   │       ├── manager.go      # MCP server lifecycle
│   │       └── adapter.go      # MCP tools → cobot Tool interface
│   ├── subagent/               # Subagent spawner and coordinator
│   │   ├── coordinator.go
│   │   ├── agent.go            # SubAgent instance
│   │   └── result.go
│   ├── acp/                    # Agent Client Protocol server
│   │   ├── server.go           # JSON-RPC 2.0 server
│   │   ├── handler.go          # ACP method handlers
│   │   ├── transport.go        # Stdio transport
│   │   └── types.go            # ACP types (match spec)
│   ├── workspace/              # Workspace discovery and management
│   │   ├── workspace.go        # Workspace struct
│   │   ├── discovery.go        # Find .cobot/ up the directory tree
│   │   └── init.go             # Workspace initialization
│   ├── skills/                 # Skill system (procedural memory)
│   │   ├── skill.go            # Skill definition
│   │   ├── loader.go           # Load skills from workspace
│   │   └── executor.go         # Execute skill steps
│   ├── scheduler/              # Cron-like task scheduler
│   │   ├── scheduler.go
│   │   └── task.go
│   └── config/                 # Configuration management
│       ├── config.go           # Config struct and loading
│       ├── loader.go           # Layered config loading
│       └── defaults.go
├── pkg/                        # Public SDK
│   ├── cobot.go                # Main entry: New(config) → Agent
│   ├── types.go                # Public types: Message, Tool, ToolCall, etc.
│   ├── options.go              # Functional options
│   ├── interfaces.go           # Public interfaces: Provider, Tool, MemoryStore
│   ├── errors.go               # Public error types
│   └── event.go                # Event types for streaming/channels
├── api/                        # ACP protocol types (shared between internal/acp and pkg)
│   ├── acp/
│   │   ├── types.go            # ACP JSON-RPC types (match spec exactly)
│   │   └── schema.go           # Schema constants
│   └── openapi/                # Future: OpenAPI spec for HTTP transport
└── tui/                        # Terminal UI (bubbletea)
    ├── app.go                  # Main TUI application
    ├── chat.go                 # Chat view
    ├── input.go                # Multi-line input
    └── theme.go                # Styling
```

### Dependency Graph

```
cmd/cobot → pkg/ + tui/
pkg/      → internal/ (via controlled exports)
cobra-desktop → pkg/ (external consumer)

internal/agent → internal/llm, internal/memory, internal/tools, internal/subagent, internal/skills
internal/tools → internal/tools/builtin, internal/tools/mcp
internal/mcp   → github.com/modelcontextprotocol/go-sdk/mcp (external)
internal/acp   → internal/agent, api/acp
```

---

## 2. Core Agent Loop

### Flow

```
User Message
    │
    ▼
┌─────────────────────────────────┐
│  1. LOAD CONTEXT                │
│  - System prompt + personality  │
│  - Workspace AGENTS.md          │
│  - L0+L1 memory (identity+facts)│
│  - Recent conversation history  │
│  - Skill definitions            │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  2. THINK (LLM call)            │
│  - Send messages to provider    │
│  - Stream response chunks       │
│  - Parse tool calls from output │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  3. DECIDE                      │
│  Tool calls?                    │
│  YES → Execute tools            │
│         ├─ Built-in tools       │
│         ├─ MCP server calls     │
│         └─ Subagent spawn       │
│  Append results as messages     │
│  NO  → Final response           │
│         └─ Save to memory       │
└────────────┬────────────────────┘
             │
             ▼
       Loop back to THINK
       (until max_turns, stop reason, or cancel)
```

### Core Types

```go
// pkg/types.go

type Role string
const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

type Message struct {
    Role       Role            `json:"role"`
    Content    string          `json:"content"`
    ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
    ToolResult *ToolResult     `json:"tool_result,omitempty"`
    Metadata   map[string]any  `json:"metadata,omitempty"`
}

type ToolCall struct {
    ID       string          `json:"id"`
    Name     string          `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
    CallID string `json:"call_id"`
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}

type StopReason string
const (
    StopEndTurn         StopReason = "end_turn"
    StopMaxTokens       StopReason = "max_tokens"
    StopMaxTurnRequests StopReason = "max_turn_requests"
    StopCancelled       StopReason = "cancelled"
    StopRefusal         StopReason = "refusal"
)
```

### Agent Config

```go
// pkg/options.go

type Config struct {
    ConfigPath  string         // --config flag
    Workspace   string         // --workspace flag
    Model       string         // provider:model format, e.g. "openai:gpt-4o"
    MaxTurns    int            // max tool-calling loops per message
    SystemPrompt string        // Override system prompt
    Memory      MemoryConfig
    Tools       []string       // enabled tool names
    Verbose     bool           // debug logging
}

type MemoryConfig struct {
    Enabled  bool
    BadgerPath string          // Override BadgerDB path
    BlevePath  string          // Override Bleve index path
}
```

---

## 3. LLM Provider System

### Interface

```go
// pkg/interfaces.go

type Provider interface {
    Name() string
    Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
    Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)
}

type ProviderRequest struct {
    Model       string     `json:"model"`
    Messages    []Message  `json:"messages"`
    Tools       []ToolDef  `json:"tools"`
    MaxTokens   int        `json:"max_tokens"`
    Temperature float64    `json:"temperature"`
}

type ProviderResponse struct {
    Content    string     `json:"content"`
    ToolCalls  []ToolCall `json:"tool_calls"`
    StopReason StopReason `json:"stop_reason"`
    Usage      Usage      `json:"usage"`
}

type ProviderChunk struct {
    Content   string     `json:"content,omitempty"`
    ToolCall  *ToolCall  `json:"tool_call,omitempty"`
    Done      bool       `json:"done"`
    Usage     *Usage     `json:"usage,omitempty"`
}

type ToolDef struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### Provider Registry

```go
// internal/llm/registry.go

type Registry struct {
    providers map[string]Provider
}

func (r *Registry) Register(name string, p Provider)
func (r *Registry) Get(name string) (Provider, error)
func (r *Registry) List() []string
```

### OpenAI Provider (First Implementation)

Supports OpenAI API, OpenRouter, and any OpenAI-compatible endpoint.

```go
// internal/llm/openai/provider.go

type OpenAIProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider
```

---

## 4. Memory System

### Architecture (MemPalace-inspired)

```
~/.cobot/workspaces/<hash>/memory/
├── badger/                     # BadgerDB KV store
│   ├── drawers/                # Raw verbatim content
│   ├── closets/                # Summaries pointing to drawers
│   ├── rooms/                  # Room metadata
│   ├── wings/                  # Wing metadata
│   └── knowledge/              # Knowledge graph triples
├── bleve/                      # Bleve search indexes
│   ├── drawers.bleve           # Full-text index on drawer content
│   └── closets.bleve           # Index on summaries
└── sessions/                   # Session transcripts
```

### Palace Structure

```
Wing (person or project)
  ├── Hall (memory type)
  │   ├── hall_facts        — decisions, locked-in choices
  │   ├── hall_events       — sessions, milestones
  │   ├── hall_discoveries  — breakthroughs, insights
  │   ├── hall_preferences  — habits, opinions
  │   └── hall_advice       — recommendations
  ├── Room (specific topic)
  │   ├── Closet (summary → points to drawer)
  │   └── Drawer (raw verbatim content)
  └── Tunnel (cross-wing connection between shared rooms)
```

### Memory Stack (L0-L3)

| Layer | Content | Size | When Loaded |
|-------|---------|------|-------------|
| L0 | Identity — who is this AI? | ~50 tokens | Always |
| L1 | Critical facts — team, projects, preferences | ~120 tokens | Always |
| L2 | Room recall — recent sessions, current project | On demand | Topic mentioned |
| L3 | Deep search — semantic query across all closets | On demand | Explicit query |

### Interfaces

```go
// pkg/interfaces.go

type MemoryStore interface {
    // Palace operations
    Store(ctx context.Context, entry *MemoryEntry) error
    Search(ctx context.Context, query *SearchQuery) ([]*SearchResult, error)

    // Wing/Room management
    GetWings(ctx context.Context) ([]*Wing, error)
    GetRooms(ctx context.Context, wingID string) ([]*Room, error)
    CreateWing(ctx context.Context, wing *Wing) error
    CreateRoom(ctx context.Context, room *Room) error

    // Drawer operations
    AddDrawer(ctx context.Context, wingID, roomID, content string) (string, error)
    GetDrawer(ctx context.Context, id string) (*Drawer, error)
    SearchDrawers(ctx context.Context, query string, opts ...SearchOption) ([]*DrawerResult, error)

    // Layer stack
    WakeUp(ctx context.Context) (string, error) // Returns L0+L1 context
}

type KnowledgeGraph interface {
    AddTriple(ctx context.Context, triple *Triple) error
    Invalidate(ctx context.Context, subject, predicate, object string, ended time.Time) error
    Query(ctx context.Context, entity string, asOf *time.Time) ([]*Triple, error)
    Timeline(ctx context.Context, entity string) ([]*Triple, error)
}
```

### Types

```go
// pkg/types.go

type Wing struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Type      string    `json:"type"`  // "person" or "project"
    Keywords  []string  `json:"keywords"`
}

type Room struct {
    ID        string    `json:"id"`
    WingID    string    `json:"wing_id"`
    Name      string    `json:"name"`
    HallType  string    `json:"hall_type"`  // facts, events, discoveries, preferences, advice
}

type Drawer struct {
    ID        string    `json:"id"`
    RoomID    string    `json:"room_id"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}

type Closet struct {
    ID        string   `json:"id"`
    RoomID    string   `json:"room_id"`
    DrawerIDs []string `json:"drawer_ids"`
    Summary   string   `json:"summary"`
}

type Triple struct {
    Subject   string     `json:"subject"`
    Predicate string     `json:"predicate"`
    Object    string     `json:"object"`
    ValidFrom time.Time  `json:"valid_from"`
    ValidTo   *time.Time `json:"valid_to,omitempty"`
}

type SearchQuery struct {
    Text     string   `json:"text"`
    WingID   string   `json:"wing_id,omitempty"`
    RoomID   string   `json:"room_id,omitempty"`
    HallType string   `json:"hall_type,omitempty"`
    Limit    int      `json:"limit,omitempty"`
}
```

---

## 5. Tool System

### Tool Interface

```go
// pkg/interfaces.go

type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage // JSON Schema
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

### Tool Registry

```go
// internal/tools/registry.go

type Registry struct {
    builtin map[string]Tool
    mcp     map[string]Tool // MCP-provided tools
}

func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, error)
func (r *Registry) List() []ToolDef
func (r *Registry) Execute(ctx context.Context, call ToolCall) (string, error)
```

### Built-in Tools

| Tool | Description |
|------|-------------|
| `filesystem_read` | Read file contents |
| `filesystem_write` | Write file contents |
| `filesystem_search` | Search files by pattern |
| `shell_exec` | Execute shell commands |
| `web_fetch` | Fetch URL content |
| `web_search` | Web search |
| `memory_search` | Search agent memory |
| `memory_store` | Store to agent memory |
| `subagent_spawn` | Spawn a subagent |
| `skill_execute` | Execute a skill |

### MCP Integration

Uses `github.com/modelcontextprotocol/go-sdk/mcp` directly.

```go
// internal/tools/mcp/manager.go

type MCPManager struct {
    client   *mcp.Client
    sessions map[string]*mcp.ClientSession
}

func (m *MCPManager) Connect(ctx context.Context, name string, config MCPServerConfig) error
func (m *MCPManager) Disconnect(ctx context.Context, name string) error
func (m *MCPManager) ListTools(ctx context.Context) ([]ToolDef, error)
func (m *MCPManager) CallTool(ctx context.Context, name string, args map[string]any) (string, error)
```

MCP tools are wrapped as `Tool` interface implementations and registered in the tool registry alongside built-in tools.

---

## 6. ACP Server

### Protocol Compliance

Full compliance with [Agent Client Protocol](https://agentclientprotocol.com) specification:
- JSON-RPC 2.0 over stdio transport
- Baseline methods: `initialize`, `authenticate`, `session/new`, `session/prompt`
- Optional methods: `session/load`, `session/set_mode`, `session/set_config_option`, `session/list`
- Client methods: `session/request_permission`, `fs/read_text_file`, `fs/write_text_file`, `terminal/*`
- Notifications: `session/update`, `session/cancel`

### Implementation

```go
// internal/acp/server.go

type ACPServer struct {
    agent    *agent.Agent
    sessions map[string]*Session
}

func NewACPServer(a *agent.Agent) *ACPServer
func (s *ACPServer) Run(ctx context.Context, transport Transport) error
```

### Transport

```go
// internal/acp/transport.go

type Transport interface {
    ReadMessage() (*jsonrpc.Message, error)
    WriteMessage(msg *jsonrpc.Message) error
    Close() error
}

type StdioTransport struct { /* reads stdin, writes stdout */ }
```

### Session Update Notifications

```go
type SessionUpdate struct {
    SessionUpdate string          `json:"sessionUpdate"` // enum
    // Fields vary by update type:
    // "agent_message_chunk" → Content ContentBlock
    // "user_message_chunk"  → Content ContentBlock
    // "tool_call"           → ToolCallID, Title, Kind, Status
    // "tool_call_update"    → ToolCallID, Status, Content
    // "plan"                → Entries []PlanEntry
    // "available_commands"  → AvailableCommands []Command
    // "current_mode_update" → ModeID string
    // "config_option_update"→ ConfigOptions []ConfigOption
}
```

---

## 7. Subagent System

### Coordinator

```go
// internal/subagent/coordinator.go

type Coordinator struct {
    parent   *agent.Agent
    agents   map[string]*SubAgent
    mu       sync.RWMutex
}

func (c *Coordinator) Spawn(ctx context.Context, config *SubAgentConfig) (*SubAgent, error)
func (c *Coordinator) Get(id string) (*SubAgent, error)
func (c *Coordinator) Gather(ctx context.Context, ids []string) ([]*SubAgentResult, error)
func (c *Coordinator) CancelAll(ctx context.Context) error
```

### SubAgent Config

```go
type SubAgentConfig struct {
    Task        string        // What the subagent should do
    Model       string        // Override model (empty = parent's model)
    Tools       []string      // Subset of parent's tools (empty = all)
    MaxTurns    int           // Max tool-calling loops
    Isolated    bool          // Own workspace sandbox
    ShareMemory bool          // Can read parent's memory
    Timeout     time.Duration // Max execution time
}

type SubAgentResult struct {
    ID       string
    Output   string
    Error    string
    Duration time.Duration
    ToolCalls int
}
```

### Lifecycle

1. Parent's LLM decides to spawn a subagent (tool call to `subagent_spawn`)
2. Coordinator creates SubAgent with restricted tools and optional shared memory
3. SubAgent runs independently, streaming events via channel
4. Parent collects results and merges into its context

---

## 8. Workspace System

### Directory Structure

```
project/
├── .cobot/
│   ├── config.yaml          # Workspace-specific config
│   ├── AGENTS.md            # Bot personality/instructions
│   ├── tools.yaml           # MCP servers, tool configuration
│   ├── context/             # Files always loaded into context
│   │   └── *.md
│   ├── memory/              # Local memory store
│   │   ├── badger/
│   │   └── bleve/
│   ├── sessions/            # Conversation history
│   │   └── *.jsonl
│   ├── skills/              # Workspace-specific skills
│   └── scheduler/
│       └── tasks.yaml
└── ...                      # Project files
```

### Workspace Discovery

When `cobot` runs in a directory, it searches upward for `.cobot/`:
1. Current directory
2. Parent directories up to root
3. Falls back to `~/.cobot/` global config only

### Config Layering

Priority (highest to lowest):
1. CLI flags (`--config`, `--workspace`, `--model`)
2. Environment variables (`COBOT_MODEL`, `COBOT_CONFIG`, `COBOT_WORKSPACE`)
3. Workspace config (`.cobot/config.yaml`)
4. Global config (`~/.cobot/config.yaml`)
5. Compiled defaults

### Config Types

```go
// internal/config/config.go

type Config struct {
    // Paths
    ConfigPath  string `yaml:"-" flag:"config"`
    Workspace   string `yaml:"workspace" flag:"workspace"`

    // Agent
    Model       string  `yaml:"model" env:"COBOT_MODEL"`
    MaxTurns    int     `yaml:"max_turns"`
    Temperature float64 `yaml:"temperature"`

    // API Keys
    APIKeys map[string]string `yaml:"api_keys"`

    // Providers
    Providers map[string]ProviderConfig `yaml:"providers"`

    // Memory
    Memory MemoryConfig `yaml:"memory"`

    // Tools
    Tools ToolsConfig `yaml:"tools"`

    // Defaults
    DefaultTools []string `yaml:"default_tools"`
}

type ProviderConfig struct {
    BaseURL string            `yaml:"base_url"`
    Headers map[string]string `yaml:"headers"`
}

type ToolsConfig struct {
    Builtin    []string                     `yaml:"builtin"`
    MCPServers map[string]MCPServerConfig   `yaml:"mcp_servers"`
}

type MCPServerConfig struct {
    Transport string            `yaml:"transport"` // "stdio" | "http" | "sse"
    Command   string            `yaml:"command"`
    Args      []string          `yaml:"args"`
    Env       map[string]string `yaml:"env"`
    URL       string            `yaml:"url"`       // for http/sse
    Headers   map[string]string `yaml:"headers"`   // for http/sse
}
```

---

## 9. CLI Design

### Commands

```
cobot                                    # Start interactive TUI
cobot chat "message"                     # One-shot message
cobot chat -w /path "message"            # With workspace override
cobot -c /path/to/config.yaml chat "hi"  # With config override

cobot model                              # List available models
cobot model set openai:gpt-4o            # Set active model

cobot tools                              # List enabled tools
cobot tools enable <name>                # Enable a tool
cobot tools disable <name>               # Disable a tool

cobot config set <key> <value>           # Set config value
cobot config get <key>                   # Get config value

cobot memory search "query"              # Search memory
cobot memory search "query" -w mywing    # Search within wing
cobot memory status                      # Palace overview
cobot memory mine <path>                 # Mine data into memory

cobot workspace init                     # Initialize .cobot/ in current dir
cobot workspace list                     # List known workspaces

cobot acp serve                          # Start ACP server (stdio mode)
cobot acp serve --http :8080             # Start ACP server (HTTP mode, future)

cobot scheduler list                     # List scheduled tasks
cobot scheduler add "daily report"       # Add scheduled task

cobot setup                              # First-time setup wizard
cobot doctor                             # Diagnose issues
cobot version                            # Show version
```

### TUI Mode

When `cobot` is run without arguments, a bubbletea TUI starts:
- Multi-line input (ctrl+enter or enter to send)
- Streaming response display with tool call indicators
- Slash commands in the input: `/new`, `/model`, `/tools`, `/memory`, `/compress`, `/skills`, `/retry`, `/undo`
- Ctrl+c interrupts current generation
- Markdown rendering for responses
- Conversation history sidebar

---

## 10. SDK Design (pkg/)

### Entry Point

```go
// pkg/cobot.go

func New(config Config) (*Agent, error)
func (a *Agent) Prompt(ctx context.Context, message string) (*Response, error)
func (a *Agent) Stream(ctx context.Context, message string) (<-chan Event, error)
func (a *Agent) RegisterTool(tool Tool) error
func (a *Agent) Memory() MemoryStore
func (a *Agent) ServeACP(ctx context.Context, addr string) error
func (a *Agent) Close() error
```

### Event Types

```go
// pkg/event.go

type Event struct {
    Type    EventType
    Content string
    ToolCall *ToolCall
    Done    bool
    Error   error
}

type EventType int
const (
    EventText Eventtype = iota
    EventToolCall
    EventToolResult
    EventDone
    EventError
)
```

### Usage Example (cobra-desktop)

```go
import cobot "github.com/cobot-agent/cobot/pkg"

func main() {
    agent, err := cobot.New(cobot.Config{
        ConfigPath: "/path/to/config.yaml",
        Workspace:  "/path/to/project",
        Model:      "openai:gpt-4o",
    })
    if err != nil { log.Fatal(err) }
    defer agent.Close()

    ch, err := agent.Stream(ctx, "explain this codebase")
    for event := range ch {
        switch event.Type {
        case cobot.EventText:
            fmt.Print(event.Content)
        case cobot.EventToolCall:
            fmt.Printf("[Tool: %s]", event.ToolCall.Name)
        case cobot.EventDone:
            fmt.Println("\n---")
        }
    }
}
```

---

## 11. Skill System

### Skill Definition

```yaml
# .cobot/skills/review.yaml
name: code-review
description: Review code for quality and bugs
trigger: "/review"
steps:
  - prompt: "Analyze the following files for code quality issues"
    tool: filesystem_search
    args:
      pattern: "**/*.go"
  - prompt: "Review each file and provide feedback"
    output: review_output
  - prompt: "Summarize the review findings"
    output: summary
```

### Skill Interface

```go
// internal/skills/skill.go

type Skill struct {
    Name        string
    Description string
    Trigger     string   // slash command
    Steps       []Step
}

type Step struct {
    Prompt string
    Tool   string
    Args   map[string]any
    Output string
}

type Executor struct {
    agent *agent.Agent
}

func (e *Executor) Execute(ctx context.Context, skill *Skill, input string) (string, error)
```

---

## 12. Scheduler

### Task Definition

```yaml
# .cobot/scheduler/tasks.yaml
tasks:
  - name: daily-summary
    schedule: "0 9 * * *"
    prompt: "Summarize what I worked on yesterday"
    output: memory
  - name: weekly-audit
    schedule: "0 17 * * 5"
    prompt: "Audit the codebase for issues"
    output: file
    output_path: ./reports/weekly-audit.md
```

### Implementation

```go
// internal/scheduler/scheduler.go

type Scheduler struct {
    agent *agent.Agent
    cron  *cron.Cron
}

func (s *Scheduler) Start(ctx context.Context) error
func (s *Scheduler) AddTask(task *Task) error
func (s *Scheduler) RemoveTask(name string) error
func (s *Scheduler) ListTasks() []*Task
```

---

## 13. Error Handling

All errors use structured error types:

```go
// pkg/errors.go

type CobotError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Cause   error  `json:"-"`
}

var (
    ErrProviderNotConfigured = &CobotError{Code: "PROVIDER_NOT_CONFIGURED"}
    ErrWorkspaceNotFound     = &CobotError{Code: "WORKSPACE_NOT_FOUND"}
    ErrToolNotFound          = &CobotError{Code: "TOOL_NOT_FOUND"}
    ErrMemorySearchFailed    = &CobotError{Code: "MEMORY_SEARCH_FAILED"}
    ErrMaxTurnsExceeded      = &CobotError{Code: "MAX_TURNS_EXCEEDED"}
    ErrAgentCancelled        = &CobotError{Code: "AGENT_CANCELLED"}
)
```

---

## 14. Testing Strategy

- **Unit tests:** Each internal package tested independently with interfaces
- **Integration tests:** Full agent loop with mock LLM provider
- **ACP conformance:** JSON-RPC message-level tests against ACP spec
- **Memory benchmarks:** BadgerDB/Bleve performance tests with realistic data volumes
- **SDK tests:** Public API tested as external consumer would use it

---

## 15. Complete Technology Stack & Package API Reference

### 15.1 CLI Framework — `github.com/spf13/cobra`

**Purpose:** Command structure, flags, subcommands, help generation, shell completions.

**Core Type: `cobra.Command`**

```go
type Command struct {
    Use   string            // "appname [flags]"
    Short string            // one-line description
    Long  string            // full description
    Run   func(cmd *Command, args []string)
    RunE  func(cmd *Command, args []string) error
    Args  PositionalArgs    // cobra.ExactArgs(n), cobra.NoArgs, etc.
}
```

**Key Methods:**

| Method | Purpose |
|--------|---------|
| `rootCmd.AddCommand(cmds...)` | Add child subcommands |
| `rootCmd.Execute()` | Main entry, uses os.Args[1:] |
| `rootCmd.ExecuteContext(ctx)` | Execute with context |
| `cmd.Flags().StringVarP(...)` | Register local flag |
| `cmd.PersistentFlags().StringVarP(...)` | Register persistent (inherited) flag |
| `cmd.MarkFlagRequired("name")` | Mark flag as required |
| `cmd.SetArgs([]string{})` | Override args (testing) |
| `cmd.SetContext(ctx)` | Attach context |

**Lifecycle Hooks (execution order):**
1. `PersistentPreRun` / `PersistentPreRunE` — inherited by children
2. `PreRun` / `PreRunE` — this command only
3. `Run` / `RunE` — main action
4. `PostRun` / `PostPostRun`
5. `PersistentPostRun` / `PersistentPostRunE` — inherited

**Flag Types:** `StringP`, `BoolP`, `IntP`, `StringSliceP`, `StringArrayP`, `CountP`, `DurationP`

**Package-level:**
```go
cobra.OnInitialize(func())   // Run before any command
cobra.CheckErr(err)          // Print error and exit
```

**Cobot usage pattern:**
```go
var rootCmd = &cobra.Command{Use: "cobot"}
var configPath string
var workspacePath string

func init() {
    rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
    rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", "", "workspace directory")
}

func main() { cobra.CheckErr(rootCmd.Execute()) }
```

---

### 15.2 TUI Framework — `github.com/charmbracelet/bubbletea/v2`

**Purpose:** Rich terminal UI with streaming responses, multiline input, chat history.

**Core Interface: `tea.Model`**

```go
type Model interface {
    Init() Cmd
    Update(msg Msg) (Model, Cmd)
    View() View
}
```

**Key Types:**

| Type | Purpose |
|------|---------|
| `tea.Cmd` | `func() Msg` — async I/O operation |
| `tea.Msg` | Any — message carrying data into Update |
| `tea.KeyPressMsg` | Key press event, match via `msg.String()` |
| `tea.WindowSizeMsg` | `{Width, Height int}` — terminal resize |
| `tea.QuitMsg` | Signal quit |

**Built-in Commands:**

```go
tea.Quit                           // Signal exit
tea.Batch(cmds...)                 // Run concurrent
tea.Sequence(cmds...)              // Run sequential
tea.Tick(d, fn)                    // Single-shot timer
tea.Every(d, fn)                   // Periodic timer
tea.ExecProcess(cmd, callback)     // Run subprocess
```

**Program Lifecycle:**

```go
p := tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen())
model, err := p.Run()
p.Send(msg)   // Inject message from outside
p.Quit()      // Quit from outside
```

**Related packages:**

| Package | Import | Purpose |
|---------|--------|---------|
| bubbles | `charm.land/bubbles/v2` | Pre-built components: `textarea`, `viewport`, `spinner`, `table`, `list` |
| lipgloss | `charm.land/lipgloss/v2` | Styling: colors, borders, padding, layout (`JoinHorizontal`, `JoinVertical`) |
| glamour | `github.com/charmbracelet/glamour` | Markdown rendering to terminal |

**Cobot TUI architecture:**
```go
type chatModel struct {
    messages  []chatMessage
    viewport  viewport.Model
    textarea  textarea.Model
    agent     *cobot.Agent
    streaming bool
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch msg.String() {
        case "ctrl+c":
            if m.streaming { return m, tea.Interrupt }
            return m, tea.Quit
        case "enter":
            if !m.textarea.Focused() { return m, nil }
            return m, sendMessage(m.textarea.Value())
        }
    case responseMsg:
        m.messages = append(m.messages, msg.chatMessage)
    }
    var cmd tea.Cmd
    m.textarea, cmd = m.textarea.Update(msg)
    return m, cmd
}
```

---

### 15.3 Embedded KV Store — `github.com/dgraph-io/badger/v4`

**Purpose:** Store memory palace data (drawers, closets, rooms, wings, knowledge graph triples).

**Open/Close:**

```go
db, err := badger.Open(badger.DefaultOptions("/path/to/badger"))
defer db.Close()
```

**High-level KV Operations:**

```go
// Write
err := db.Update(func(txn *badger.Txn) error {
    return txn.Set([]byte("key"), []byte("value"))
})

// Read
err := db.View(func(txn *badger.Txn) error {
    item, err := txn.Get([]byte("key"))
    if errors.Is(err, badger.ErrKeyNotFound) { return nil }
    return item.Value(func(val []byte) error {
        fmt.Println(string(val))
        return nil
    })
})
```

**Item Access (within transaction):**

```go
item.Key()           // []byte — valid until Next() or txn ends
item.KeyCopy(nil)    // []byte — safe copy
item.Value(fn)       // callback-based read
item.ValueCopy(nil)  // []byte — safe copy
item.UserMeta()      // byte
item.ExpiresAt()     // uint64
```

**Prefix Scanning (for palace namespaces):**

```go
err := db.View(func(txn *badger.Txn) error {
    opts := badger.DefaultIteratorOptions
    opts.Prefix = []byte("wing:")
    it := txn.NewIterator(opts)
    defer it.Close()
    for it.Rewind(); it.Valid(); it.Next() {
        item := it.Item()
        k := item.KeyCopy(nil)
        v, _ := item.ValueCopy(nil)
    }
    return nil
})
```

**WriteBatch (bulk writes):**

```go
wb := db.NewWriteBatch()
wb.Set([]byte("key1"), []byte("val1"))
wb.Set([]byte("key2"), []byte("val2"))
wb.Flush()
```

**Entry with TTL:**

```go
err := txn.SetEntry(badger.NewEntry([]byte("key"), []byte("val")).WithTTL(time.Hour))
```

**Garbage Collection:**

```go
db.RunValueLogGC(0.5)  // discardRatio 0.5
lsm, vlog := db.Size() // check sizes
```

**Key Errors:** `ErrKeyNotFound`, `ErrTxnTooBig`, `ErrConflict`, `ErrDBClosed`

**Cobot key prefix design:**

| Prefix | Content |
|--------|---------|
| `wing:{id}` | Wing metadata (JSON) |
| `room:{wingID}:{roomID}` | Room metadata (JSON) |
| `drawer:{id}` | Raw verbatim content |
| `closet:{roomID}:{id}` | Summary pointing to drawers |
| `kg:{subject}:{predicate}:{object}` | Knowledge graph triple |
| `session:{id}` | Session transcript |

---

### 15.4 Full-text & Vector Search — `github.com/blevesearch/bleve/v2`

**Purpose:** Search memory drawers and closets with full-text, faceted, and vector similarity queries.

**Index Creation:**

```go
mapping := bleve.NewIndexMapping()
index, err := bleve.New("/path/to/index.bleve", mapping)
// or memory-only:
index, err := bleve.NewMemOnly(mapping)
// or open existing:
index, err := bleve.Open("/path/to/index.bleve")
defer index.Close()
```

**Document Indexing:**

```go
err := index.Index("drawer:abc123", DrawerDocument{
    Content:   "verbatim content here...",
    WingID:    "wing_kai",
    RoomID:    "room_auth-migration",
    HallType:  "facts",
    CreatedAt: time.Now(),
})
err := index.Delete("drawer:abc123")
```

**Field Mapping:**

```go
docMapping := bleve.NewDocumentMapping()

contentField := bleve.NewTextFieldMapping()
contentField.Analyzer = "en"
docMapping.AddFieldMappingsAt("content", contentField)

wingField := bleve.NewKeywordFieldMapping()
docMapping.AddFieldMappingsAt("wing_id", wingField)

roomField := bleve.NewKeywordFieldMapping()
docMapping.AddFieldMappingsAt("room_id", roomField)

hallField := bleve.NewKeywordFieldMapping()
docMapping.AddFieldMappingsAt("hall_type", hallField)

dateField := bleve.NewDateTimeFieldMapping()
docMapping.AddFieldMappingsAt("created_at", dateField)

// Vector field (requires `vectors` build tag + FAISS)
vecField := bleve.NewVectorFieldMapping()
vecField.Dims = 1536
vecField.Similarity = "cosine"
docMapping.AddFieldMappingsAt("embedding", vecField)

mapping.AddDocumentMapping("drawer", docMapping)
```

**Search Queries:**

```go
// Full-text search
q := bleve.NewMatchQuery("auth decision")

// Filtered by wing and room
wingQ := bleve.NewTermQuery("wing_kai")
wingQ.SetField("wing_id")
roomQ := bleve.NewTermQuery("room_auth-migration")
roomQ.SetField("room_id")
boolQ := bleve.NewBooleanQuery()
boolQ.AddMust(wingQ, roomQ)
boolQ.AddMust(bleve.NewMatchQuery("auth decision"))

req := bleve.NewSearchRequest(boolQ)
req.Highlight = bleve.NewHighlight()
req.Fields = []string{"content", "wing_id", "room_id"}
req.Size = 10

result, err := index.Search(req)
// result.Hits — []*search.DocumentMatch
// result.Total — total matches
// hit.Fields["content"] — stored field value
// hit.Fragments["content"] — highlighted snippets
```

**Vector (KNN) Search:**

```go
req := bleve.NewSearchRequest(bleve.NewMatchNoneQuery())
req.AddKNN("embedding", queryVector, 10, 1.0)
// Hybrid: text + vector
req = bleve.NewSearchRequest(bleve.NewMatchQuery("auth"))
req.AddKNN("embedding", queryVector, 10, 1.0)
req.Score = bleve.ScoreRRF // Reciprocal Rank Fusion
```

**Batch Indexing:**

```go
batch := index.NewBatch()
batch.Index("doc1", data1)
batch.Index("doc2", data2)
index.Batch(batch)
```

**Faceted Search (memory analytics):**

```go
req.AddFacet("wings", bleve.NewFacetRequest("wing_id", 20))
req.AddFacet("halls", bleve.NewFacetRequest("hall_type", 5))
```

**Cobot document types:**

```go
type DrawerDocument struct {
    Content   string     `json:"content"`
    WingID    string     `json:"wing_id"`
    RoomID    string     `json:"room_id"`
    HallType  string     `json:"hall_type"`
    CreatedAt time.Time  `json:"created_at"`
    Embedding []float32  `json:"embedding,omitempty"`
}

type ClosetDocument struct {
    Summary   string     `json:"summary"`
    WingID    string     `json:"wing_id"`
    RoomID    string     `json:"room_id"`
    DrawerIDs []string   `json:"drawer_ids"`
}
```

---

### 15.5 MCP SDK — `github.com/modelcontextprotocol/go-sdk/mcp`

**Purpose:** MCP client (connect to external tool servers) and MCP server (expose cobot tools to others).

**Client Usage (connect to MCP servers):**

```go
client := mcp.NewClient(&mcp.Implementation{
    Name: "cobot", Version: "0.1.0",
}, nil)

// Stdio transport (subprocess)
session, err := client.Connect(ctx, &mcp.CommandTransport{
    Command: exec.Command("npx", "-y", "@modelcontextprotocol/server-filesystem", "/path"),
}, nil)
defer session.Close()

// List available tools
tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})

// Call a tool
result, err := session.CallTool(ctx, &mcp.CallToolParams{
    Name:      "read_file",
    Arguments: map[string]any{"path": "/tmp/test.txt"},
})
// result.Content — []mcp.Content (TextContent, etc.)
// result.IsError — bool

// Read a resource
res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "file:///tmp/test.txt"})

// Get a prompt
prompt, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
    Name: "code_review",
    Arguments: map[string]string{"language": "go"},
})

// Paging iterator (Go 1.23+)
for tool, err := range session.Tools(ctx, nil) { ... }
```

**Server Usage (expose cobot as MCP server):**

```go
server := mcp.NewServer(&mcp.Implementation{
    Name: "cobot-mcp", Version: "0.1.0",
}, nil)

// Generic tool registration (auto-generates JSON schema from struct tags)
mcp.AddTool(server, &mcp.Tool{
    Name: "memory_search", Description: "Search agent memory",
}, func(ctx context.Context, req *mcp.CallToolRequest, input MemorySearchInput) (
    *mcp.CallToolResult, MemorySearchOutput, error,
) {
    results, err := memory.Search(ctx, input.Query)
    return nil, MemorySearchOutput{Results: results}, err
})

// Run over stdio
server.Run(ctx, &mcp.StdioTransport{})
```

**Key Types:**

| Type | Purpose |
|------|---------|
| `mcp.Client` | Long-lived client, create sessions |
| `mcp.ClientSession` | Per-connection, call tools/resources/prompts |
| `mcp.Server` | Long-lived server, register features |
| `mcp.ServerSession` | Per-connection, access client capabilities |
| `mcp.Tool` | `{Name, Description, InputSchema, OutputSchema}` |
| `mcp.CallToolParams` | `{Name, Arguments}` |
| `mcp.CallToolResult` | `{Content []Content, IsError bool}` |
| `mcp.TextContent` | `{Text string}` — implements Content |
| `mcp.Resource` | `{URI, Name, Description, MIMEType}` |
| `mcp.Prompt` | `{Name, Description, Arguments}` |
| `mcp.Implementation` | `{Name, Version string}` |

**Transport Types:**

| Transport | Side | Usage |
|-----------|------|-------|
| `mcp.StdioTransport` | Server | stdin/stdout |
| `mcp.CommandTransport` | Client | Launch subprocess, talk via stdin/stdout |
| `mcp.StreamableClientTransport` | Client | HTTP transport |
| `mcp.StreamableServerTransport` | Server | HTTP transport |
| `mcp.SSEClientTransport` | Client | SSE transport (deprecated) |
| `mcp.InMemoryTransport` | Both | Testing |

**JSON-RPC Types (public facade):**

```go
import "github.com/modelcontextprotocol/go-sdk/jsonrpc"

jsonrpc.EncodeMessage(msg)  // []byte
jsonrpc.DecodeMessage(data) // Message
jsonrpc.MakeID(v)           // ID
```

---

### 15.6 JSON-RPC 2.0 — `github.com/sourcegraph/jsonrpc2`

**Purpose:** ACP server implementation (JSON-RPC 2.0 over stdio). Used alongside MCP go-sdk's internal jsonrpc.

**Core Handler:**

```go
type Handler interface {
    Handle(ctx context.Context, conn *Conn, req *Request)
}
```

**Connection:**

```go
stream := jsonrpc2.NewPlainObjectStream(rwc) // rwc = combined stdin+stdout
conn := jsonrpc2.NewConn(ctx, stream, handler)

// Server responds manually
conn.Reply(ctx, req.ID, result)
conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{Code: -32601, Message: "method not found"})

// Send notification
conn.Notify(ctx, "session/update", params)

// Lifecycle
<-conn.DisconnectNotify()
conn.Close()
```

**Request/Response Types:**

```go
type Request struct {
    Method string
    Params *json.RawMessage
    ID     ID
    Notif  bool // true = notification
}

type Response struct {
    ID     *json.RawMessage
    Result *json.RawMessage
    Error  *Error
}

type Error struct {
    Code    int64
    Message string
    Data    *json.RawMessage
}
```

**Stream Construction for stdio:**

```go
type stdioRWC struct {
    io.Reader
    io.Writer
    io.Closer
}

stream := jsonrpc2.NewPlainObjectStream(&stdioRWC{
    Reader: os.Stdin,
    Writer: os.Stdout,
    Closer: os.Stdin,
})
```

**Async Handler (for concurrent request processing):**

```go
handler := jsonrpc2.AsyncHandler(myHandler{})
```

**Error-returning convenience:**

```go
handler := jsonrpc2.HandlerWithError(func(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
    switch req.Method {
    case "initialize":
        return InitializeResponse{...}, nil
    case "session/new":
        return NewSessionResponse{...}, nil
    default:
        return nil, &jsonrpc2.Error{Code: -32601, Message: "method not found"}
    }
})
```

---

### 15.7 Cron Scheduler — `github.com/robfig/cron/v3`

**Purpose:** Scheduled automations (daily reports, weekly audits, etc.).

**Core API:**

```go
c := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
id, _ := c.AddFunc("0 30 9 * * *", func() { /* daily at 9:30 */ })
c.Start()
defer c.Stop()
c.Remove(id)
```

**Key Methods:**

| Method | Purpose |
|--------|---------|
| `cron.New(opts...)` | Create scheduler |
| `c.AddFunc(spec, fn)` | Add function with cron spec |
| `c.AddJob(spec, job)` | Add Job interface |
| `c.Schedule(schedule, job)` | Add with custom Schedule |
| `c.Start()` | Start in goroutine |
| `c.Stop()` | Stop, returns context |
| `c.Entries()` | List entries |
| `c.Remove(id)` | Remove entry |

**Options:** `cron.WithSeconds()`, `cron.WithLocation(loc)`, `cron.WithLogger(l)`, `cron.WithChain(wrappers...)`

**Job Wrappers:** `cron.Recover(logger)`, `cron.SkipIfStillRunning(logger)`, `cron.DelayIfStillRunning(logger)`

**Cron Spec:** 5-field (min hour dom month dow) or 6-field with `WithSeconds()`. Special: `@hourly`, `@daily`, `@weekly`, `@monthly`, `@every 1h30m`

**Cobot usage:**

```go
func (s *Scheduler) loadTasks(path string) error {
    for _, task := range config.Tasks {
        id, _ := s.cron.AddFunc(task.Schedule, func() {
            s.agent.Prompt(context.Background(), task.Prompt)
        })
        s.ids[task.Name] = id
    }
    return nil
}
```

---

### 15.8 Additional Dependencies

| Dependency | Purpose | Import |
|------------|---------|--------|
| YAML parser | Config files | `gopkg.in/yaml.v3` |
| JSON Schema | Tool parameter validation | `github.com/invopop/jsonschema` |
| HTTP client | OpenAI API calls | `net/http` (stdlib) |
| WebSocket | Future streaming transports | `github.com/coder/websocket` |
| Logging | Structured logging | `log/slog` (stdlib) |
| Testing | Table-driven, mocks | `testing` (stdlib) |
| Embedding generation | Vector embeddings for memory | `internal/llm/openai` (reuse provider) |

---

### 15.9 Go Module Dependencies (go.mod)

```
module github.com/cobot-agent/cobot

go 1.24

require (
    github.com/spf13/cobra                    v1.9.1
    charm.land/bubbletea/v2                   v2.0.2
    charm.land/bubbles/v2                     v2.0.2
    charm.land/lipgloss/v2                    v2.0.2
    github.com/charmbracelet/glamour          v0.9.1
    github.com/dgraph-io/badger/v4            v4.6.0
    github.com/blevesearch/bleve/v2           v2.5.7
    github.com/modelcontextprotocol/go-sdk    v1.5.0
    github.com/sourcegraph/jsonrpc2           v0.2.0
    github.com/robfig/cron/v3                 v3.0.1
    gopkg.in/yaml.v3                          v3.0.1
    github.com/invopop/jsonschema             v0.13.0
)
```

---

## 16. Phased Implementation Plan

### Phase 1: Foundation
- Project structure, go.mod, config system
- LLM provider interface + OpenAI provider
- Basic agent loop (think → act → observe)
- Built-in tools: filesystem, shell

### Phase 2: Memory
- BadgerDB store for drawers/closets/rooms/wings
- Bleve search index
- Palace structure (wing/room/closet/drawer)
- L0-L3 memory stack
- Knowledge graph (temporal triples)
- Memory mining

### Phase 3: Protocols
- MCP integration (using go-sdk)
- ACP server (JSON-RPC 2.0 over stdio, full spec)
- Subagent coordinator

### Phase 4: CLI & SDK
- Cobra CLI structure with all commands
- Bubbletea TUI
- SDK (pkg/) with public API
- Config layering with path overrides

### Phase 5: Advanced
- Skill system
- Scheduler
- Additional LLM providers (Anthropic, etc.)
- Additional MCP transport (HTTP, SSE)
- Performance optimization
