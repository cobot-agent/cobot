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

## 15. Key Dependencies

| Dependency | Purpose | Version |
|------------|---------|---------|
| `github.com/spf13/cobra` | CLI framework | latest |
| `github.com/charmbracelet/bubbletea` | TUI framework | latest |
| `github.com/dgraph-io/badger/v4` | Embedded KV store | v4 |
| `github.com/blevesearch/bleve/v2` | Full-text + vector search | v2 |
| `github.com/modelcontextprotocol/go-sdk/mcp` | MCP client/server | latest |
| `github.com/robfig/cron/v3` | Cron scheduler | v3 |
| `github.com/sourcegraph/jsonrpc2` | JSON-RPC 2.0 | latest |

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
