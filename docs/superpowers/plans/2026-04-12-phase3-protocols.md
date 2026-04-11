# Phase 3: Protocols — MCP, ACP, SubAgent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the protocol layer — MCP client integration (connect to external tool servers via go-sdk), ACP server (JSON-RPC 2.0 over stdio per Agent Client Protocol spec), and subagent coordinator — enabling cobot to interoperate with MCP servers, ACP-compatible editors, and spawn sub-agents.

**Architecture:** Three independent subsystems wired together. MCP uses the official `go-sdk` to connect to external tool servers and expose cobot's own tools. ACP uses `jrpc2` for JSON-RPC 2.0 over stdio with handler-based method dispatch, exposing cobot as an ACP-compliant agent. SubAgent coordinator spawns child agent instances with restricted tool sets and optional shared memory.

**Tech Stack:** `github.com/modelcontextprotocol/go-sdk/mcp` (MCP), `github.com/creachadair/jrpc2` + `jrpc2/channel` + `jrpc2/handler` (ACP), `github.com/modelcontextprotocol/go-sdk/jsonrpc` (JSON-RPC types), Go 1.26

**Design Spec:** `docs/specs/2026-04-12-cobot-agent-system-design.md` Sections 5, 6, 7, 15.5, 15.6

---

## Phase 2 Prerequisites — Known Issues & Technical Debt

These issues were identified during Phase 2 review. They do not block Phase 3 implementation but must be tracked for future resolution. Where relevant, Phase 3 code should avoid exacerbating them.

### Critical (Fix before production)

| # | Issue | Location | Impact | Mitigation |
|---|-------|----------|--------|------------|
| P2-1 | **Bleve open/close per operation** — `indexDrawer()` and `searchDrawers()` open and close the Bleve index on every call. No connection pool or persistent index handle. | `internal/memory/search.go:22-51,53-60` | Performance bottleneck under load. Each open/close has non-trivial overhead. | Phase 5 optimization task. Phase 3 code that calls Store.Search() should batch when possible. |
| P2-2 | **TOCTOU race in MemoryStoreTool** — `findOrCreateWing()` and `findOrCreateRoom()` check-then-create in two separate transactions. Concurrent tool calls could create duplicate wings/rooms. | `internal/memory/memory_tool.go:111-145` | Data integrity issue if agent makes parallel `memory_store` calls with same wing/room name. | Add mutex or use BadgerDB transaction-level conflict detection. Not blocking since current agent loop is sequential. |
| P2-3 | **Store() partial failure** — If `AddDrawer` succeeds but `indexDrawer` fails, the drawer exists in BadgerDB but is unsearchable. No rollback or compensation. | `internal/memory/store.go:57-77` | Silent data loss for search. Drawer exists but can't be found via Search(). | Should wrap in compensating transaction or add background reconciliation. |

### Moderate (Fix in Phase 4-5)

| # | Issue | Location | Impact |
|---|-------|----------|--------|
| P2-4 | **WakeUp only L0+L1** — L2 (recent conversation context) and L3 (deep search triggered by relevance) not implemented. Only identity + facts. | `internal/memory/layers.go` | WakeUp context is minimal. L2/L3 needed for rich agent memory. |
| P2-5 | **No auto-summarization** — Closet.Summary must be manually provided. No logic to auto-generate summaries from Drawer contents. | `internal/memory/closets.go` | Closets are only useful if someone/something writes summaries. |
| P2-6 | **Knowledge Graph has no Delete** — Only Invalidate (sets ValidTo). No physical deletion of triples. | `internal/memory/knowledge.go` | Storage growth over time for superseded triples. |
| P2-7 | **Memory not integrated with Agent Loop** — Memory system is standalone. Agent loop in `internal/agent/loop.go` does not call WakeUp or use memory tools. | `internal/agent/loop.go` | Agent has no memory in conversations. Must wire up in Phase 3 or 4. |
| P2-8 | **No memory mining** — `miner.go` planned but not implemented. No way to extract facts from conversations into the memory palace automatically. | Design spec section 4 | Conversations happen but no facts are persisted to memory. |

### Pre-conditions for Phase 3

| Condition | Status | Notes |
|-----------|--------|-------|
| `internal/agent/agent.go` — Agent struct has `config`, `provider`, `tools`, `session` | ✅ Ready | Phase 3 will add memory store field |
| `internal/tools/registry.go` — Tool registry with Register/Get/ExecuteParallel | ✅ Ready | MCP tools will be registered here |
| `pkg/interfaces.go` — Tool, Provider, MemoryStore, KnowledgeGraph interfaces | ✅ Ready | ACP will use these types |
| `pkg/types.go` — All public types including StopReason constants | ✅ Ready | ACP StopReason maps to cobot.StopReason |
| `internal/config/` — Config loading from YAML | ✅ Ready | MCP server configs loaded from workspace config |
| `internal/workspace/` — Workspace discovery with DataDir | ✅ Ready | Memory store initialized from workspace.DataDir |
| Memory store wired into Agent | ❌ Not yet | Task 1 below will add it |
| `go.mod` has jrpc2 + go-sdk deps | ❌ Not yet | Task 2 below will add them |

---

## File Structure

```
api/
└── acp/
    └── types.go              # ACP protocol types (shared between internal/acp and tests)

internal/
├── agent/
│   ├── agent.go              # MODIFY: add memoryStore field, SetMemoryStore, memory tools registration
│   ├── loop.go               # MODIFY: inject WakeUp context into system prompt
│   └── context.go            # MODIFY: build system prompt with memory context
├── acp/
│   ├── server.go             # ACPServer struct, NewACPServer, Run over stdio
│   ├── handler.go            # ACP method handlers: initialize, session/new, session/prompt, session/cancel
│   ├── session.go            # ACP session management (map of sessionID → Session)
│   └── types.go              # Internal ACP types (SessionUpdate builders)
├── mcp/
│   ├── manager.go            # MCPManager: Connect, Disconnect, ListTools, CallTool
│   └── adapter.go            # MCPToolAdapter: wraps MCP tools as cobot.Tool interface
├── subagent/
│   ├── coordinator.go        # Coordinator: Spawn, Get, Gather, CancelAll
│   └── types.go              # SubAgentConfig, SubAgentResult
└── tools/
    └── builtin/
        └── subagent.go       # SubAgentSpawnTool (built-in tool)

cmd/cobot/
└── acp.go                    # MODIFY: add `cobot acp serve` CLI command
```

---

### Task 1: Wire Memory into Agent

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/context.go`
- Test: `internal/agent/loop_test.go`

**Goal:** Add `memoryStore` to Agent, auto-register memory tools, inject WakeUp context into system prompt.

- [ ] **Step 1: Modify `internal/agent/agent.go`**

Add `memoryStore` field and `SetMemoryStore` method. When SetMemoryStore is called, also register the memory tools.

Current agent.go:
```go
type Agent struct {
    config   *cobot.Config
    provider cobot.Provider
    tools    *tools.Registry
    session  *Session
}
```

Add `memoryStore` field and method:

```go
import (
    "github.com/cobot-agent/cobot/internal/memory"
    "github.com/cobot-agent/cobot/internal/tools"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type Agent struct {
    config      *cobot.Config
    provider    cobot.Provider
    tools       *tools.Registry
    session     *Session
    memoryStore *memory.Store
}

func (a *Agent) SetMemoryStore(s *memory.Store) {
    a.memoryStore = s
    a.tools.Register(memory.NewMemorySearchTool(s))
    a.tools.Register(memory.NewMemoryStoreTool(s))
}

func (a *Agent) MemoryStore() *memory.Store {
    return a.memoryStore
}
```

- [ ] **Step 2: Modify `internal/agent/context.go`**

Current context.go only has `buildMessages`. Add system prompt building with memory:

```go
package agent

import (
    "context"
    "fmt"

    cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) buildSystemPrompt(ctx context.Context) string {
    base := "You are Cobot, a personal AI assistant."
    if a.memoryStore == nil {
        return base
    }
    memCtx, err := a.memoryStore.WakeUp(ctx)
    if err != nil {
        return base
    }
    if memCtx != "" {
        return memCtx
    }
    return base
}

func (a *Agent) buildMessages(ctx context.Context) []cobot.Message {
    msgs := a.session.Messages()
    sysPrompt := a.buildSystemPrompt(ctx)
    result := make([]cobot.Message, 0, len(msgs)+1)
    result = append(result, cobot.Message{Role: cobot.RoleSystem, Content: sysPrompt})
    result = append(result, msgs...)
    return result
}
```

- [ ] **Step 3: Modify `internal/agent/loop.go`**

Replace `a.session.Messages()` calls with `a.buildMessages(ctx)` in both `Prompt` and `Stream`:

In `Prompt()`, change line `msgs := a.session.Messages()` to `msgs := a.buildMessages(ctx)`.

In `Stream()`, change `msgs := a.session.Messages()` to `msgs := a.buildMessages(ctx)`.

The full change in Prompt:
```go
func (a *Agent) Prompt(ctx context.Context, message string) (*cobot.ProviderResponse, error) {
    if a.provider == nil {
        return nil, cobot.ErrProviderNotConfigured
    }

    a.session.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

    for turn := 0; turn < a.config.MaxTurns; turn++ {
        msgs := a.buildMessages(ctx)
        req := &cobot.ProviderRequest{
            Model:    a.config.Model,
            Messages: msgs,
            Tools:    a.tools.ToolDefs(),
        }
        // ... rest unchanged
```

Same change in Stream.

- [ ] **Step 4: Update existing tests**

Read the current `internal/agent/loop_test.go` and `internal/agent/e2e_test.go`. Verify they still pass with the new `buildMessages` signature (it now takes `ctx`).

- [ ] **Step 5: Run tests**

Run: `go test ./internal/agent/... -v -count=1`

- [ ] **Step 6: Commit**

```bash
git add internal/agent/
git commit -m "feat: wire memory store into agent, inject WakeUp context into system prompt"
```

---

### Task 2: Add Dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add MCP go-sdk and jrpc2**

```bash
cd /Users/muk/Work/github.com/cobot-agent/cobot
go get github.com/modelcontextprotocol/go-sdk/mcp@latest
go get github.com/creachadair/jrpc2@latest
go mod tidy
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add MCP go-sdk and jrpc2 for Phase 3"
```

---

### Task 3: ACP Protocol Types

**Files:**
- Create: `api/acp/types.go`
- Test: `api/acp/types_test.go`

**Goal:** Define all ACP protocol types matching the official schema at https://agentclientprotocol.com/protocol/schema. These are pure data types with JSON tags — no business logic.

- [ ] **Step 1: Create `api/acp/types.go`**

```go
package acp

import "encoding/json"

type ProtocolVersion = int

type Implementation struct {
    Name    string `json:"name"`
    Title   string `json:"title,omitempty"`
    Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
    Fs       *FileSystemCapabilities `json:"fs,omitempty"`
    Terminal bool                    `json:"terminal,omitempty"`
}

type FileSystemCapabilities struct {
    ReadTextFile  bool `json:"readTextFile"`
    WriteTextFile bool `json:"writeTextFile"`
}

type AgentCapabilities struct {
    LoadSession       bool               `json:"loadSession"`
    PromptCapabilities *PromptCapabilities `json:"promptCapabilities,omitempty"`
    MCPCapabilities   *MCPCapabilities   `json:"mcpCapabilities,omitempty"`
    SessionCapabilities json.RawMessage   `json:"sessionCapabilities,omitempty"`
}

type PromptCapabilities struct {
    Image           bool `json:"image"`
    Audio           bool `json:"audio"`
    EmbeddedContext bool `json:"embeddedContext"`
}

type MCPCapabilities struct {
    HTTP bool `json:"http"`
    SSE  bool `json:"sse"`
}

type AuthMethod struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}

type InitializeRequest struct {
    ProtocolVersion  ProtocolVersion    `json:"protocolVersion"`
    ClientCapabilities ClientCapabilities `json:"clientCapabilities,omitempty"`
    ClientInfo       *Implementation     `json:"clientInfo,omitempty"`
}

type InitializeResponse struct {
    ProtocolVersion  ProtocolVersion    `json:"protocolVersion"`
    AgentCapabilities AgentCapabilities  `json:"agentCapabilities"`
    AgentInfo        *Implementation     `json:"agentInfo,omitempty"`
    AuthMethods      []AuthMethod        `json:"authMethods,omitempty"`
}

type MCPServer struct {
    Type    string        `json:"type,omitempty"`
    Name    string        `json:"name"`
    Command string        `json:"command,omitempty"`
    Args    []string      `json:"args,omitempty"`
    Env     []EnvVariable `json:"env,omitempty"`
    URL     string        `json:"url,omitempty"`
    Headers []HTTPHeader  `json:"headers,omitempty"`
}

type EnvVariable struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type HTTPHeader struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type NewSessionRequest struct {
    CWD       string      `json:"cwd"`
    MCPServers []MCPServer `json:"mcpServers"`
}

type NewSessionResponse struct {
    SessionID    string              `json:"sessionId"`
    ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
    Modes        *SessionModeState   `json:"modes,omitempty"`
}

type PromptRequest struct {
    SessionID string         `json:"sessionId"`
    Prompt    []ContentBlock `json:"prompt"`
}

type PromptResponse struct {
    StopReason string `json:"stopReason"`
}

type CancelNotification struct {
    SessionID string `json:"sessionId"`
}

type ContentBlock struct {
    Type     string          `json:"type"`
    Text     string          `json:"text,omitempty"`
    Resource *ResourceContent `json:"resource,omitempty"`
    URI      string          `json:"uri,omitempty"`
}

type ResourceContent struct {
    URI      string `json:"uri"`
    MIMEType string `json:"mimeType,omitempty"`
    Text     string `json:"text,omitempty"`
}

type SessionUpdateNotification struct {
    SessionID string        `json:"sessionId"`
    Update    SessionUpdate `json:"update"`
}

type SessionUpdate struct {
    SessionUpdate string          `json:"sessionUpdate"`
    Content       *ContentBlock   `json:"content,omitempty"`
    ToolCallID    string          `json:"toolCallId,omitempty"`
    Title         string          `json:"title,omitempty"`
    Kind          string          `json:"kind,omitempty"`
    Status        string          `json:"status,omitempty"`
    Entries       []PlanEntry     `json:"entries,omitempty"`
    AvailableCommands []AvailableCommand `json:"availableCommands,omitempty"`
    ModeID        string          `json:"modeId,omitempty"`
    ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
    ToolCallContent []ToolCallContentItem `json:"content,omitempty"`
}

type PlanEntry struct {
    Content  string `json:"content"`
    Priority string `json:"priority"`
    Status   string `json:"status"`
}

type AvailableCommand struct {
    Name        string `json:"name"`
    Description string `json:"description"`
}

type SessionConfigOption struct {
    ID     string   `json:"id"`
    Name   string   `json:"name"`
    Values []ConfigValue `json:"values"`
}

type ConfigValue struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
}

type SessionModeState struct {
    AvailableModes []SessionMode `json:"availableModes"`
    CurrentModeID  string        `json:"currentModeId"`
}

type SessionMode struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}

type ToolCallContentItem struct {
    Type    string        `json:"type"`
    Content *ContentBlock `json:"content,omitempty"`
}

type PermissionOption struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
}

type RequestPermissionRequest struct {
    SessionID string          `json:"sessionId"`
    ToolCall  ToolCallUpdate  `json:"toolCall"`
    Options   []PermissionOption `json:"options"`
}

type RequestPermissionResponse struct {
    Outcome string `json:"outcome"`
}

type ToolCallUpdate struct {
    ToolCallID string `json:"toolCallId"`
    Title      string `json:"title"`
    Kind       string `json:"kind"`
    Status     string `json:"status"`
}

type LoadSessionRequest struct {
    SessionID  string      `json:"sessionId"`
    CWD       string      `json:"cwd"`
    MCPServers []MCPServer `json:"mcpServers"`
}

type LoadSessionResponse struct {
    ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
    Modes        *SessionModeState      `json:"modes,omitempty"`
}
```

- [ ] **Step 2: Create `api/acp/types_test.go`**

Test JSON round-trip for key types:

```go
package acp

import (
    "encoding/json"
    "testing"
)

func TestInitializeRequestJSON(t *testing.T) {
    req := InitializeRequest{
        ProtocolVersion: 1,
        ClientInfo: &Implementation{Name: "test-client", Version: "1.0.0"},
        ClientCapabilities: ClientCapabilities{
            Fs: &FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
            Terminal: true,
        },
    }
    data, err := json.Marshal(req)
    if err != nil {
        t.Fatal(err)
    }
    var got InitializeRequest
    if err := json.Unmarshal(data, &got); err != nil {
        t.Fatal(err)
    }
    if got.ProtocolVersion != 1 {
        t.Errorf("expected version 1, got %d", got.ProtocolVersion)
    }
    if got.ClientInfo.Name != "test-client" {
        t.Errorf("expected test-client, got %s", got.ClientInfo.Name)
    }
    if !got.ClientCapabilities.Fs.ReadTextFile {
        t.Error("expected ReadTextFile true")
    }
}

func TestInitializeResponseJSON(t *testing.T) {
    resp := InitializeResponse{
        ProtocolVersion: 1,
        AgentCapabilities: AgentCapabilities{
            LoadSession: true,
            PromptCapabilities: &PromptCapabilities{Image: false},
        },
        AgentInfo: &Implementation{Name: "cobot", Version: "0.1.0"},
    }
    data, err := json.Marshal(resp)
    if err != nil {
        t.Fatal(err)
    }
    if string(data) == "" {
        t.Error("expected non-empty JSON")
    }
    var got InitializeResponse
    json.Unmarshal(data, &got)
    if !got.AgentCapabilities.LoadSession {
        t.Error("expected LoadSession true")
    }
}

func TestSessionUpdateNotificationJSON(t *testing.T) {
    n := SessionUpdateNotification{
        SessionID: "sess_123",
        Update: SessionUpdate{
            SessionUpdate: "agent_message_chunk",
            Content: &ContentBlock{Type: "text", Text: "hello"},
        },
    }
    data, err := json.Marshal(n)
    if err != nil {
        t.Fatal(err)
    }
    var got SessionUpdateNotification
    json.Unmarshal(data, &got)
    if got.Update.SessionUpdate != "agent_message_chunk" {
        t.Errorf("expected agent_message_chunk, got %s", got.Update.SessionUpdate)
    }
    if got.Update.Content.Text != "hello" {
        t.Error("expected text 'hello'")
    }
}

func TestPromptRequestJSON(t *testing.T) {
    req := PromptRequest{
        SessionID: "sess_123",
        Prompt: []ContentBlock{
            {Type: "text", Text: "analyze this code"},
            {Type: "resource_link", URI: "file:///tmp/test.go"},
        },
    }
    data, _ := json.Marshal(req)
    var got PromptRequest
    json.Unmarshal(data, &got)
    if len(got.Prompt) != 2 {
        t.Errorf("expected 2 content blocks, got %d", len(got.Prompt))
    }
}

func TestMCPServerStdio(t *testing.T) {
    s := MCPServer{
        Name:    "filesystem",
        Command: "/usr/local/bin/mcp-fs",
        Args:    []string{"--stdio"},
        Env:     []EnvVariable{{Name: "KEY", Value: "val"}},
    }
    data, _ := json.Marshal(s)
    var got MCPServer
    json.Unmarshal(data, &got)
    if got.Name != "filesystem" {
        t.Errorf("expected filesystem, got %s", got.Name)
    }
    if len(got.Args) != 1 {
        t.Errorf("expected 1 arg, got %d", len(got.Args))
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./api/acp/... -v`

- [ ] **Step 4: Commit**

```bash
git add api/
git commit -m "feat: add ACP protocol types matching official spec"
```

---

### Task 4: ACP Server — Core + Handlers

**Files:**
- Create: `internal/acp/server.go`
- Create: `internal/acp/handler.go`
- Create: `internal/acp/session.go`
- Create: `internal/acp/types.go`
- Test: `internal/acp/server_test.go`

**Goal:** Build the ACP server using jrpc2. It reads JSON-RPC from stdin, dispatches to handlers, and writes responses to stdout.

- [ ] **Step 1: Create `internal/acp/types.go`** — Internal helper types

```go
package acp

type Session struct {
    ID       string
    CWD      string
    CancelFn context.CancelFunc
}

type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
    return &SessionStore{sessions: make(map[string]*Session)}
}

func (s *SessionStore) Put(sess *Session) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.sessions[sess.ID] = sess
}

func (s *SessionStore) Get(id string) (*Session, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    sess, ok := s.sessions[id]
    return sess, ok
}

func (s *SessionStore) Delete(id string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.sessions, id)
}
```

Use `"context"`, `"sync"`, `"crypto/rand"`, `"encoding/hex"` imports.

- [ ] **Step 2: Create `internal/acp/session.go`** — Session helpers

Generate session IDs, manage session lifecycle:

```go
package acp

func newSessionID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return "sess_" + hex.EncodeToString(b)
}
```

- [ ] **Step 3: Create `internal/acp/server.go`** — ACPServer struct + Run

```go
package acp

import (
    "context"
    "os"

    "github.com/creachadair/jrpc2"
    "github.com/creachadair/jrpc2/channel"
    "github.com/creachadair/jrpc2/handler"

    "github.com/cobot-agent/cobot/internal/agent"
    acpapi "github.com/cobot-agent/cobot/api/acp"
)

type ACPServer struct {
    agent      *agent.Agent
    sessions   *SessionStore
    jrpcServer *jrpc2.Server
}

func NewACPServer(a *agent.Agent) *ACPServer {
    return &ACPServer{
        agent:    a,
        sessions: NewSessionStore(),
    }
}

func (s *ACPServer) Run(ctx context.Context) error {
    assigner := handler.ServiceMap{
        "session": handler.Map{
            "new":    handler.New(s.handleSessionNew),
            "prompt": handler.New(s.handleSessionPrompt),
            "cancel": handler.New(s.handleSessionCancel),
        },
    }

    topHandlers := handler.Map{
        "initialize":  handler.New(s.handleInitialize),
        "authenticate": handler.New(s.handleAuthenticate),
    }

    merged := make(handler.Map)
    for k, v := range topHandlers {
        merged[k] = v
    }
    for k, v := range assigner {
        merged[k] = v
    }

    s.jrpcServer = jrpc2.NewServer(merged, &jrpc2.ServerOptions{
        AllowPush:   true,
        Concurrency: 0,
    })

    ch := channel.Line(os.Stdin, os.Stdout)
    s.jrpcServer.Start(ch)

    go func() {
        <-ctx.Done()
        s.jrpcServer.Stop()
    }()

    return s.jrpcServer.Wait()
}

func (s *ACPServer) notify(ctx context.Context, method string, params any) {
    srv := jrpc2.ServerFromContext(ctx)
    if srv != nil {
        srv.Notify(ctx, method, params)
    }
}
```

Note: `handler.ServiceMap` produces methods like `"session.new"`, but ACP uses `"session/new"`. We need to use `handler.Map` with explicit keys instead. Fix: use a flat `handler.Map` with slash-separated keys:

```go
assigner := handler.Map{
    "initialize":    handler.New(s.handleInitialize),
    "authenticate":  handler.New(s.handleAuthenticate),
    "session/new":   handler.New(s.handleSessionNew),
    "session/prompt": handler.New(s.handleSessionPrompt),
    "session/cancel": handler.New(s.handleSessionCancel),
}
```

- [ ] **Step 4: Create `internal/acp/handler.go`** — Method handlers

```go
package acp

import (
    "context"

    acpapi "github.com/cobot-agent/cobot/api/acp"
    "github.com/cobot-agent/cobot/pkg"
)

func (s *ACPServer) handleInitialize(ctx context.Context, req acpapi.InitializeRequest) (acpapi.InitializeResponse, error) {
    return acpapi.InitializeResponse{
        ProtocolVersion: req.ProtocolVersion,
        AgentCapabilities: acpapi.AgentCapabilities{
            LoadSession: false,
            PromptCapabilities: &acpapi.PromptCapabilities{
                Image:           false,
                Audio:           false,
                EmbeddedContext: false,
            },
        },
        AgentInfo: &acpapi.Implementation{
            Name:    "cobot",
            Title:   "Cobot Agent",
            Version: "0.1.0",
        },
        AuthMethods: []acpapi.AuthMethod{},
    }, nil
}

func (s *ACPServer) handleAuthenticate(ctx context.Context, req map[string]any) (map[string]any, error) {
    return map[string]any{}, nil
}

func (s *ACPServer) handleSessionNew(ctx context.Context, req acpapi.NewSessionRequest) (acpapi.NewSessionResponse, error) {
    id := newSessionID()
    ctx, cancel := context.WithCancel(context.Background())
    sess := &Session{ID: id, CWD: req.CWD, CancelFn: cancel}
    s.sessions.Put(sess)
    return acpapi.NewSessionResponse{SessionID: id}, nil
}

func (s *ACPServer) handleSessionPrompt(ctx context.Context, req acpapi.PromptRequest) (acpapi.PromptResponse, error) {
    _, ok := s.sessions.Get(req.SessionID)
    if !ok {
        return acpapi.PromptResponse{}, jrpc2.Errorf(jrpc2.InvalidParams, "session %q not found", req.SessionID)
    }

    var text string
    for _, block := range req.Prompt {
        if block.Type == "text" {
            text += block.Text
        }
    }

    s.notify(ctx, "session/update", acpapi.SessionUpdateNotification{
        SessionID: req.SessionID,
        Update: acpapi.SessionUpdate{
            SessionUpdate: "agent_message_chunk",
            Content:       &acpapi.ContentBlock{Type: "text", Text: "Processing..."},
        },
    })

    resp, err := s.agent.Prompt(ctx, text)
    if err != nil {
        return acpapi.PromptResponse{StopReason: string(cobot.StopEndTurn)}, nil
    }

    s.notify(ctx, "session/update", acpapi.SessionUpdateNotification{
        SessionID: req.SessionID,
        Update: acpapi.SessionUpdate{
            SessionUpdate: "agent_message_chunk",
            Content:       &acpapi.ContentBlock{Type: "text", Text: resp.Content},
        },
    })

    return acpapi.PromptResponse{StopReason: string(cobot.StopEndTurn)}, nil
}

func (s *ACPServer) handleSessionCancel(ctx context.Context, req acpapi.CancelNotification) (any, error) {
    if sess, ok := s.sessions.Get(req.SessionID); ok {
        if sess.CancelFn != nil {
            sess.CancelFn()
        }
    }
    return nil, nil
}
```

Wait — the handler needs to import `pkg` as `cobot` not as `pkg`. Also `jrpc2` needs to be imported. Let me fix the imports:

```go
import (
    "context"

    jrpc2 "github.com/creachadair/jrpc2"

    acpapi "github.com/cobot-agent/cobot/api/acp"
    cobot "github.com/cobot-agent/cobot/pkg"
)
```

And the stop reason should be: `cobot.StopEndTurn` converted to string: `string(cobot.StopEndTurn)`.

- [ ] **Step 5: Create `internal/acp/server_test.go`** — In-memory test using jrpc2's `server.NewLocal`

```go
package acp

import (
    "context"
    "testing"

    "github.com/creachadair/jrpc2"
    "github.com/creachadair/jrpc2/handler"
    "github.com/creachadair/jrpc2/server"

    "github.com/cobot-agent/cobot/internal/agent"
    acpapi "github.com/cobot-agent/cobot/api/acp"
    cobot "github.com/cobot-agent/cobot/pkg"
)

func newTestACPServer(t *testing.T) (*ACPServer, *server.Local) {
    t.Helper()
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    a := agent.New(cfg)
    s := NewACPServer(a)

    assigner := handler.Map{
        "initialize":    handler.New(s.handleInitialize),
        "authenticate":  handler.New(s.handleAuthenticate),
        "session/new":   handler.New(s.handleSessionNew),
        "session/prompt": handler.New(s.handleSessionPrompt),
        "session/cancel": handler.New(s.handleSessionCancel),
    }

    loc := server.NewLocal(assigner, nil)
    t.Cleanup(func() { loc.Close() })
    return s, loc
}

func TestHandleInitialize(t *testing.T) {
    _, loc := newTestACPServer(t)

    var resp acpapi.InitializeResponse
    err := loc.Client.CallResult(context.Background(), "initialize", acpapi.InitializeRequest{
        ProtocolVersion: 1,
        ClientInfo:      &acpapi.Implementation{Name: "test", Version: "1.0"},
    }, &resp)
    if err != nil {
        t.Fatal(err)
    }
    if resp.ProtocolVersion != 1 {
        t.Errorf("expected version 1, got %d", resp.ProtocolVersion)
    }
    if resp.AgentInfo.Name != "cobot" {
        t.Errorf("expected cobot, got %s", resp.AgentInfo.Name)
    }
}

func TestHandleSessionNew(t *testing.T) {
    _, loc := newTestACPServer(t)

    var resp acpapi.NewSessionResponse
    err := loc.Client.CallResult(context.Background(), "session/new", acpapi.NewSessionRequest{
        CWD:       "/tmp/test",
        MCPServers: []acpapi.MCPServer{},
    }, &resp)
    if err != nil {
        t.Fatal(err)
    }
    if resp.SessionID == "" {
        t.Error("expected session ID")
    }
}

func TestHandleSessionPromptNoProvider(t *testing.T) {
    s, loc := newTestACPServer(t)

    var sessResp acpapi.NewSessionResponse
    loc.Client.CallResult(context.Background(), "session/new", acpapi.NewSessionRequest{
        CWD: "/tmp", MCPServers: []acpapi.MCPServer{},
    }, &sessResp)

    var promptResp acpapi.PromptResponse
    err := loc.Client.CallResult(context.Background(), "session/prompt", acpapi.PromptRequest{
        SessionID: sessResp.SessionID,
        Prompt:    []acpapi.ContentBlock{{Type: "text", Text: "hello"}},
    }, &promptResp)
    if err != nil {
        t.Logf("expected error (no provider): %v", err)
    }
}

func TestHandleSessionPromptInvalidSession(t *testing.T) {
    _, loc := newTestACPServer(t)

    var resp acpapi.PromptResponse
    err := loc.Client.CallResult(context.Background(), "session/prompt", acpapi.PromptRequest{
        SessionID: "nonexistent",
        Prompt:    []acpapi.ContentBlock{{Type: "text", Text: "hello"}},
    }, &resp)
    if err == nil {
        t.Error("expected error for nonexistent session")
    }
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/acp/... -v`

- [ ] **Step 7: Commit**

```bash
git add internal/acp/ api/
git commit -m "feat: add ACP server with initialize, session/new, session/prompt handlers"
```

---

### Task 5: MCP Client Manager

**Files:**
- Create: `internal/mcp/manager.go`
- Create: `internal/mcp/adapter.go`
- Test: `internal/mcp/manager_test.go`

**Goal:** MCPManager connects to external MCP tool servers (subprocess via stdio) using the official go-sdk, discovers their tools, wraps them as `cobot.Tool` interface, and registers them in the tool registry.

- [ ] **Step 1: Create `internal/mcp/adapter.go`** — Wrap MCP tools as cobot.Tool

```go
package mcp

import (
    "context"
    "encoding/json"
    "fmt"

    cobot "github.com/cobot-agent/cobot/pkg"
)

type MCPToolAdapter struct {
    name        string
    description string
    schema      json.RawMessage
    callFunc    func(ctx context.Context, args json.RawMessage) (string, error)
}

func (t *MCPToolAdapter) Name() string        { return t.name }
func (t *MCPToolAdapter) Description() string { return t.description }
func (t *MCPToolAdapter) Parameters() json.RawMessage { return t.schema }

func (t *MCPToolAdapter) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    return t.callFunc(ctx, args)
}

var _ cobot.Tool = (*MCPToolAdapter)(nil)
```

- [ ] **Step 2: Create `internal/mcp/manager.go`** — MCPManager

```go
package mcp

import (
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "sync"

    "github.com/modelcontextprotocol/go-sdk/mcp"

    cobot "github.com/cobot-agent/cobot/pkg"
)

type MCPServerConfig struct {
    Name    string
    Command string
    Args    []string
    Env     map[string]string
}

type MCPManager struct {
    mu       sync.RWMutex
    client   *mcp.Client
    sessions map[string]*mcp.ClientSession
}

func NewMCPManager() *MCPManager {
    return &MCPManager{
        client:   mcp.NewClient(&mcp.Implementation{Name: "cobot", Version: "0.1.0"}, nil),
        sessions: make(map[string]*mcp.ClientSession),
    }
}

func (m *MCPManager) Connect(ctx context.Context, config MCPServerConfig) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    cmd := exec.CommandContext(ctx, config.Command, config.Args...)
    if len(config.Env) > 0 {
        for k, v := range config.Env {
            cmd.Env = append(cmd.Environ(), k+"="+v)
        }
    }

    transport := &mcp.CommandTransport{Command: cmd}
    session, err := m.client.Connect(ctx, transport, nil)
    if err != nil {
        return fmt.Errorf("connect to MCP server %q: %w", config.Name, err)
    }

    m.sessions[config.Name] = session
    return nil
}

func (m *MCPManager) Disconnect(ctx context.Context, name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    session, ok := m.sessions[name]
    if !ok {
        return fmt.Errorf("MCP server %q not found", name)
    }
    err := session.Close()
    delete(m.sessions, name)
    return err
}

func (m *MCPManager) ListTools(ctx context.Context, name string) ([]cobot.ToolDef, error) {
    m.mu.RLock()
    session, ok := m.sessions[name]
    m.mu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("MCP server %q not found", name)
    }

    result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
    if err != nil {
        return nil, err
    }

    var defs []cobot.ToolDef
    for _, tool := range result.Tools {
        schema, _ := json.Marshal(tool.InputSchema)
        defs = append(defs, cobot.ToolDef{
            Name:        tool.Name,
            Description: tool.Description,
            Parameters:  json.RawMessage(schema),
        })
    }
    return defs, nil
}

func (m *MCPManager) CallTool(ctx context.Context, name string, toolName string, args json.RawMessage) (string, error) {
    m.mu.RLock()
    session, ok := m.sessions[name]
    m.mu.RUnlock()
    if !ok {
        return "", fmt.Errorf("MCP server %q not found", name)
    }

    var arguments map[string]any
    if err := json.Unmarshal(args, &arguments); err != nil {
        return "", err
    }

    result, err := session.CallTool(ctx, &mcp.CallToolParams{
        Name:      toolName,
        Arguments: arguments,
    })
    if err != nil {
        return "", err
    }

    var output string
    for _, content := range result.Content {
        if text, ok := content.(mcp.TextContent); ok {
            output += text.Text
        }
    }
    if result.IsError {
        return output, fmt.Errorf("MCP tool error: %s", output)
    }
    return output, nil
}

func (m *MCPManager) ToolAdapters(ctx context.Context, serverName string) ([]cobot.Tool, error) {
    defs, err := m.ListTools(ctx, serverName)
    if err != nil {
        return nil, err
    }

    var tools []cobot.Tool
    for _, def := range defs {
        toolName := def.Name
        srvName := serverName
        adapter := &MCPToolAdapter{
            name:        def.Name,
            description: def.Description,
            schema:      def.Parameters,
            callFunc: func(ctx context.Context, args json.RawMessage) (string, error) {
                return m.CallTool(ctx, srvName, toolName, args)
            },
        }
        tools = append(tools, adapter)
    }
    return tools, nil
}

func (m *MCPManager) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    var errs []error
    for name, session := range m.sessions {
        if err := session.Close(); err != nil {
            errs = append(errs, err)
        }
        delete(m.sessions, name)
    }
    if len(errs) > 0 {
        return errs[0]
    }
    return nil
}
```

Note: The exact `mcp.CommandTransport` type and API may differ from what's shown. Check the go-sdk source after `go get` and adjust accordingly. The key pattern is: create client → connect via transport → list tools / call tool.

- [ ] **Step 3: Create `internal/mcp/manager_test.go`**

Since we can't test real MCP servers without subprocess, test the adapter and manager construction:

```go
package mcp

import (
    "context"
    "encoding/json"
    "testing"

    cobot "github.com/cobot-agent/cobot/pkg"
)

func TestNewMCPManager(t *testing.T) {
    m := NewMCPManager()
    if m == nil {
        t.Fatal("expected manager")
    }
}

func TestMCPToolAdapter(t *testing.T) {
    called := false
    adapter := &MCPToolAdapter{
        name:        "test_tool",
        description: "A test tool",
        schema:      json.RawMessage(`{"type":"object"}`),
        callFunc: func(ctx context.Context, args json.RawMessage) (string, error) {
            called = true
            return "result", nil
        },
    }

    if adapter.Name() != "test_tool" {
        t.Errorf("expected test_tool, got %s", adapter.Name())
    }
    if adapter.Description() != "A test tool" {
        t.Errorf("unexpected description: %s", adapter.Description())
    }
    if string(adapter.Parameters()) != `{"type":"object"}` {
        t.Errorf("unexpected schema: %s", adapter.Parameters())
    }

    var _ cobot.Tool = adapter

    result, err := adapter.Execute(context.Background(), json.RawMessage(`{}`))
    if err != nil {
        t.Fatal(err)
    }
    if result != "result" {
        t.Errorf("expected result, got %s", result)
    }
    if !called {
        t.Error("expected callFunc to be called")
    }
}

func TestMCPManagerConnectInvalid(t *testing.T) {
    m := NewMCPManager()
    err := m.Connect(context.Background(), MCPServerConfig{
        Name:    "bad",
        Command: "/nonexistent/binary",
    })
    if err == nil {
        t.Error("expected error connecting to invalid server")
    }
}

func TestMCPManagerDisconnectNonexistent(t *testing.T) {
    m := NewMCPManager()
    err := m.Disconnect(context.Background(), "nonexistent")
    if err == nil {
        t.Error("expected error for nonexistent server")
    }
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mcp/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/
git commit -m "feat: add MCP client manager with tool adapter"
```

---

### Task 6: SubAgent Coordinator

**Files:**
- Create: `internal/subagent/coordinator.go`
- Create: `internal/subagent/types.go`
- Test: `internal/subagent/coordinator_test.go`

**Goal:** Coordinator spawns sub-agents with restricted tool sets, runs them independently, and gathers results.

- [ ] **Step 1: Create `internal/subagent/types.go`**

```go
package subagent

import (
    "time"

    cobot "github.com/cobot-agent/cobot/pkg"
)

type Config struct {
    Task        string
    Model       string
    Tools       []string
    MaxTurns    int
    Timeout     time.Duration
    ShareMemory bool
}

type Result struct {
    ID        string
    Output    string
    Error     string
    Duration  time.Duration
    ToolCalls int
}

type SubAgent struct {
    id     string
    config *Config
    agent  interface {
        Prompt(ctx context.Context, message string) (*cobot.ProviderResponse, error)
    }
    result *Result
    done   chan struct{}
}
```

Wait — `SubAgent.agent` type should be the actual `*agent.Agent` but that creates an import cycle. Use the `cobot.Provider` interface + tools instead. Actually, the cleanest approach: create a fresh `agent.Agent` in the coordinator:

```go
package subagent

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/cobot-agent/cobot/internal/agent"
    "github.com/cobot-agent/cobot/internal/tools"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type Config struct {
    Task        string
    Model       string
    ToolNames   []string
    MaxTurns    int
    Timeout     time.Duration
    ShareMemory bool
}

type Result struct {
    ID        string
    Output    string
    Error     string
    Duration  time.Duration
    ToolCalls int
}

type SubAgent struct {
    ID     string
    config *Config
    result *Result
    done   chan struct{}
}

func (s *SubAgent) Done() <-chan struct{} { return s.done }

func (s *SubAgent) Result() *Result { return s.result }
```

- [ ] **Step 2: Create `internal/subagent/coordinator.go`**

```go
package subagent

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "sync"
    "time"

    "github.com/cobot-agent/cobot/internal/agent"
    "github.com/cobot-agent/cobot/internal/tools"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type Coordinator struct {
    parent     *agent.Agent
    mu         sync.RWMutex
    subagents  map[string]*SubAgent
}

func NewCoordinator(parent *agent.Agent) *Coordinator {
    return &Coordinator{
        parent:    parent,
        subagents: make(map[string]*SubAgent),
    }
}

func (c *Coordinator) Spawn(ctx context.Context, config *Config) (*SubAgent, error) {
    id := newSubAgentID()
    sa := &SubAgent{
        ID:     id,
        config: config,
        done:   make(chan struct{}),
    }

    c.mu.Lock()
    c.subagents[id] = sa
    c.mu.Unlock()

    go c.run(ctx, sa)

    return sa, nil
}

func (c *Coordinator) Get(id string) (*SubAgent, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    sa, ok := c.subagents[id]
    return sa, ok
}

func (c *Coordinator) Gather(ctx context.Context, ids []string) []*Result {
    var results []*Result
    for _, id := range ids {
        if sa, ok := c.Get(id); ok {
            select {
            case <-sa.Done():
                results = append(results, sa.Result())
            case <-ctx.Done():
                results = append(results, &Result{ID: id, Error: "timeout"})
            }
        }
    }
    return results
}

func (c *Coordinator) CancelAll() {
    c.mu.RLock()
    defer c.mu.RUnlock()
    for _, sa := range c.subagents {
        select {
        case <-sa.Done():
        default:
            close(sa.done)
        }
    }
}

func (c *Coordinator) run(ctx context.Context, sa *SubAgent) {
    defer close(sa.done)

    start := time.Now()
    timeout := sa.config.Timeout
    if timeout == 0 {
        timeout = 5 * time.Minute
    }

    subCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    cfg := &cobot.Config{
        Model:    c.parent.Config().Model,
        MaxTurns: sa.config.MaxTurns,
    }
    if sa.config.Model != "" {
        cfg.Model = sa.config.Model
    }
    if cfg.MaxTurns == 0 {
        cfg.MaxTurns = 5
    }

    childAgent := agent.New(cfg)

    reg := tools.NewRegistry()
    if len(sa.config.ToolNames) == 0 {
        for _, def := range c.parent.ToolRegistry().ToolDefs() {
            if t, err := c.parent.ToolRegistry().Get(def.Name); err == nil {
                reg.Register(t)
            }
        }
    } else {
        for _, name := range sa.config.ToolNames {
            if t, err := c.parent.ToolRegistry().Get(name); err == nil {
                reg.Register(t)
            }
        }
    }
    childAgent.SetToolRegistry(reg)

    if c.parent.Provider() != nil {
        childAgent.SetProvider(c.parent.Provider())
    }

    resp, err := childAgent.Prompt(subCtx, sa.config.Task)
    duration := time.Since(start)

    sa.result = &Result{
        ID:       sa.ID,
        Duration: duration,
    }

    if err != nil {
        sa.result.Error = err.Error()
        return
    }

    if resp != nil {
        sa.result.Output = resp.Content
        sa.result.ToolCalls = len(resp.ToolCalls)
    }
}

func newSubAgentID() string {
    b := make([]byte, 8)
    rand.Read(b)
    return "sub_" + hex.EncodeToString(b)
}
```

This requires adding some accessor methods to `agent.Agent`:
- `Config() *cobot.Config` — returns the config
- `Provider() cobot.Provider` — returns the provider
- `SetToolRegistry(r *tools.Registry)` — sets the tool registry

Add these to `internal/agent/agent.go`:

```go
func (a *Agent) Config() *cobot.Config      { return a.config }
func (a *Agent) Provider() cobot.Provider    { return a.provider }
func (a *Agent) SetToolRegistry(r *tools.Registry) { a.tools = r }
```

- [ ] **Step 3: Create `internal/subagent/coordinator_test.go`**

```go
package subagent

import (
    "context"
    "testing"
    "time"

    "github.com/cobot-agent/cobot/internal/agent"
    cobot "github.com/cobot-agent/cobot/pkg"
)

func TestNewCoordinator(t *testing.T) {
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    parent := agent.New(cfg)
    coord := NewCoordinator(parent)
    if coord == nil {
        t.Fatal("expected coordinator")
    }
}

func TestSpawnNoProvider(t *testing.T) {
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    parent := agent.New(cfg)
    coord := NewCoordinator(parent)

    sa, err := coord.Spawn(context.Background(), &Config{
        Task:     "test task",
        MaxTurns: 1,
        Timeout:  5 * time.Second,
    })
    if err != nil {
        t.Fatal(err)
    }
    if sa.ID == "" {
        t.Error("expected sub-agent ID")
    }

    select {
    case <-sa.Done():
        r := sa.Result()
        if r.Error == "" {
            t.Error("expected error (no provider)")
        }
    case <-time.After(10 * time.Second):
        t.Fatal("timeout waiting for sub-agent")
    }
}

func TestGather(t *testing.T) {
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    parent := agent.New(cfg)
    coord := NewCoordinator(parent)

    sa, _ := coord.Spawn(context.Background(), &Config{
        Task:    "test",
        Timeout: 5 * time.Second,
    })

    results := coord.Gather(context.Background(), []string{sa.ID})
    if len(results) != 1 {
        t.Fatalf("expected 1 result, got %d", len(results))
    }
    if results[0].ID != sa.ID {
        t.Errorf("expected ID %s, got %s", sa.ID, results[0].ID)
    }
}

func TestGetNonexistent(t *testing.T) {
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    parent := agent.New(cfg)
    coord := NewCoordinator(parent)

    _, ok := coord.Get("nonexistent")
    if ok {
        t.Error("expected not found")
    }
}
```

- [ ] **Step 4: Add accessor methods to agent.go**

Add to `internal/agent/agent.go`:

```go
func (a *Agent) Config() *cobot.Config                { return a.config }
func (a *Agent) Provider() cobot.Provider              { return a.provider }
func (a *Agent) SetToolRegistry(r *tools.Registry)     { a.tools = r }
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/subagent/... ./internal/agent/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/subagent/ internal/agent/agent.go
git commit -m "feat: add subagent coordinator with spawn/gather/cancel"
```

---

### Task 7: SubAgent Spawn Tool

**Files:**
- Create: `internal/tools/builtin/subagent.go`
- Test: `internal/tools/builtin/subagent_test.go`

**Goal:** Built-in tool that the agent can call to spawn sub-agents.

- [ ] **Step 1: Create `internal/tools/builtin/subagent.go`**

```go
package builtin

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/cobot-agent/cobot/internal/subagent"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type subagentSpawnArgs struct {
    Task      string   `json:"task"`
    Model     string   `json:"model,omitempty"`
    Tools     []string `json:"tools,omitempty"`
    MaxTurns  int      `json:"max_turns,omitempty"`
    Timeout   int      `json:"timeout_seconds,omitempty"`
}

type SubAgentSpawnTool struct {
    coordinator *subagent.Coordinator
}

func NewSubAgentSpawnTool(c *subagent.Coordinator) *SubAgentSpawnTool {
    return &SubAgentSpawnTool{coordinator: c}
}

func (t *SubAgentSpawnTool) Name() string { return "subagent_spawn" }
func (t *SubAgentSpawnTool) Description() string {
    return "Spawn a sub-agent to handle a task independently"
}
func (t *SubAgentSpawnTool) Parameters() json.RawMessage {
    return json.RawMessage(`{"type":"object","properties":{"task":{"type":"string","description":"The task for the sub-agent"},"model":{"type":"string","description":"Model override (optional)"},"tools":{"type":"array","items":{"type":"string"},"description":"Subset of tool names (optional, empty=all)"},"max_turns":{"type":"integer","description":"Max tool-calling turns (default 5)"},"timeout_seconds":{"type":"integer","description":"Timeout in seconds (default 300)"}},"required":["task"]}`)
}

func (t *SubAgentSpawnTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var a subagentSpawnArgs
    if err := json.Unmarshal(args, &a); err != nil {
        return "", err
    }

    timeout := 5 * time.Minute
    if a.Timeout > 0 {
        timeout = time.Duration(a.Timeout) * time.Second
    }
    maxTurns := a.MaxTurns
    if maxTurns == 0 {
        maxTurns = 5
    }

    sa, err := t.coordinator.Spawn(ctx, &subagent.Config{
        Task:      a.Task,
        Model:     a.Model,
        ToolNames: a.Tools,
        MaxTurns:  maxTurns,
        Timeout:   timeout,
    })
    if err != nil {
        return "", err
    }

    select {
    case <-sa.Done():
        r := sa.Result()
        if r.Error != "" {
            return fmt.Sprintf("Sub-agent %s failed: %s", r.ID, r.Error), nil
        }
        return fmt.Sprintf("Sub-agent %s completed (took %s, %d tool calls):\n%s", r.ID, r.Duration.Round(time.Millisecond), r.ToolCalls, r.Output), nil
    case <-ctx.Done():
        return "", ctx.Err()
    }
}

var _ cobot.Tool = (*SubAgentSpawnTool)(nil)
```

- [ ] **Step 2: Create `internal/tools/builtin/subagent_test.go`**

```go
package builtin

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/cobot-agent/cobot/internal/agent"
    "github.com/cobot-agent/cobot/internal/subagent"
    cobot "github.com/cobot-agent/cobot/pkg"
)

func TestSubAgentSpawnTool(t *testing.T) {
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    parent := agent.New(cfg)
    coord := subagent.NewCoordinator(parent)
    tool := NewSubAgentSpawnTool(coord)

    if tool.Name() != "subagent_spawn" {
        t.Errorf("expected subagent_spawn, got %s", tool.Name())
    }

    result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"test task"}`))
    if err != nil {
        t.Fatal(err)
    }
    if result == "" {
        t.Error("expected non-empty result")
    }
}

func TestSubAgentSpawnToolInvalidArgs(t *testing.T) {
    cfg := &cobot.Config{Model: "test", MaxTurns: 1}
    parent := agent.New(cfg)
    coord := subagent.NewCoordinator(parent)
    tool := NewSubAgentSpawnTool(coord)

    _, err := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
    if err == nil {
        t.Error("expected error for invalid JSON")
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/tools/builtin/... -v -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/tools/builtin/subagent.go internal/tools/builtin/subagent_test.go
git commit -m "feat: add subagent_spawn built-in tool"
```

---

### Task 8: CLI — `cobot acp serve` Command

**Files:**
- Modify: `cmd/cobot/root.go` — add acp subcommand
- Create: `cmd/cobot/acp.go`

**Goal:** Add `cobot acp serve` CLI command that starts the ACP server.

- [ ] **Step 1: Create `cmd/cobot/acp.go`**

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/spf13/cobra"
    "github.com/cobot-agent/cobot/internal/acp"
    "github.com/cobot-agent/cobot/internal/agent"
    cobot "github.com/cobot-agent/cobot/pkg"
)

var acpCmd = &cobra.Command{
    Use:   "acp",
    Short: "ACP server commands",
}

var acpServeCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start ACP server (stdio mode)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        a := agent.New(cfg)
        srv := acp.NewACPServer(a)

        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
        defer stop()

        return srv.Run(ctx)
    },
}

func init() {
    acpCmd.AddCommand(acpServeCmd)
    rootCmd.AddCommand(acpCmd)
}
```

Wait — `loadConfig()` is defined in `root.go`. Need to check if it's accessible. It is, since it's in the same package.

Also need to make sure `acp.go` imports correctly. The `acp` package needs the `agent` package. Check import path.

- [ ] **Step 2: Run build**

Run: `go build ./cmd/cobot/...`

- [ ] **Step 3: Commit**

```bash
git add cmd/cobot/
git commit -m "feat: add cobot acp serve CLI command"
```

---

### Task 9: Final Verification

- [ ] **Step 1:** `go build ./...`
- [ ] **Step 2:** `go vet ./...`
- [ ] **Step 3:** `go test ./... -count=1`
- [ ] **Step 4:** Verify all test counts. Expected:
  - `api/acp` — 5 tests
  - `internal/acp` — 4 tests
  - `internal/agent` — existing + new memory integration tests
  - `internal/mcp` — 4 tests
  - `internal/subagent` — 4 tests
  - `internal/tools/builtin` — existing + 2 new subagent tests
  - All other packages unchanged
- [ ] **Step 5:** Fix any issues found during verification
- [ ] **Step 6:** Final commit if fixes needed
