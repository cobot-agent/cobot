# Phase 4: CLI & SDK Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the CLI with all missing commands (tools, config, memory), enhance the SDK public API (pkg/) to expose memory/ACP/subagent, add config layering with environment variable support, and implement the Bubbletea TUI for interactive mode.

**Architecture:** CLI commands delegate to internal packages via thin Cobra handlers. SDK (`pkg/`) wraps `AgentCore` interface with higher-level methods exposing memory, ACP, and subagent. Config layering merges defaults < global config < workspace config < env vars < CLI flags. TUI uses Bubbletea with a chat model, streaming display, and slash commands.

**Tech Stack:** Cobra (CLI), Bubbletea (TUI), Bubbles/Lipgloss (TUI widgets/styling), Glamour (Markdown rendering), `gopkg.in/yaml.v3` (config), `os_expand` for env vars

**Design Spec:** `docs/specs/2026-04-12-cobot-agent-system-design.md` Sections 8, 9, 10

---

## Phase 3 Prerequisites — Outstanding Issues

From Phase 3 code review, tracked for future resolution:

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| P2-1 | Critical | Bleve open/close per operation, no connection pool | `internal/memory/search.go` |
| P2-2 | Critical | TOCTOU race in MemoryStoreTool findOrCreate | `internal/memory/memory_tool.go` |
| P2-3 | Critical | Store() partial failure (AddDrawer ok, indexDrawer fail) | `internal/memory/store.go` |
| P2-4 | Moderate | WakeUp only L0+L1 | `internal/memory/layers.go` |
| P2-5 | Moderate | No auto-summarization of Drawers | `internal/memory/closets.go` |
| P2-6 | Moderate | Knowledge graph no physical delete | `internal/memory/knowledge.go` |
| P2-8 | Moderate | No memory mining from conversations | Design spec section 4 |
| P3-I2 | Important | MCPManager.Close() swallows errors | `internal/mcp/manager.go` |
| P3-I3 | Important | buildSystemPrompt swallows WakeUp errors | `internal/agent/context.go` |
| P3-I4 | Important | ListTools uses full lock instead of RLock | `internal/mcp/manager.go` |
| P3-I5 | Important | CancelAll doesn't cancel subagent contexts | `internal/subagent/coordinator.go` |
| P3-I6 | Important | KG key collision with colon delimiter | `internal/memory/knowledge.go` |

---

## File Structure

```
cmd/cobot/
├── root.go              # MODIFY: add version command, env var expansion
├── chat.go              # MODIFY: register all tools, wire memory
├── model.go             # EXISTS
├── workspace.go         # EXISTS
├── acp.go               # EXISTS
├── tools.go             # NEW: tools list command
├── config_cmd.go        # NEW: config get/set commands
├── memory_cmd.go        # NEW: memory search/status commands
└── tui.go               # NEW: Bubbletea TUI (default no-arg mode)

pkg/
├── cobot.go             # MODIFY: add Memory(), ServeACP(), SetMemoryStore()
├── interfaces.go        # EXISTS
├── types.go             # EXISTS
├── errors.go            # EXISTS
├── options.go           # MODIFY: add ProviderConfig, ToolsConfig, MemoryConfig fields
└── cobot_test.go        # MODIFY: add tests for new SDK methods

internal/config/
├── config.go            # MODIFY: add layering (defaults < global < workspace < env < flags)
├── defaults.go          # EXISTS
└── config_test.go       # MODIFY: add layering tests
```

---

### Task 1: Config Layering — Env Vars + Workspace Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `pkg/options.go`
- Test: `internal/config/config_test.go`

**Goal:** Implement full config layering: defaults → global config → workspace `.cobot/config.yaml` → env vars → CLI flags.

- [ ] **Step 1: Add fields to Config struct** (`pkg/options.go`)

Add `ProviderConfig`, `ToolsConfig`, `Temperature`, and `MemoryConfig` to Config:

```go
type Config struct {
    ConfigPath   string                    `yaml:"config_path"`
    Workspace    string                    `yaml:"workspace"`
    Model        string                    `yaml:"model"`
    MaxTurns     int                       `yaml:"max_turns"`
    Temperature  float64                   `yaml:"temperature"`
    SystemPrompt string                    `yaml:"system_prompt"`
    Verbose      bool                      `yaml:"verbose"`
    APIKeys      map[string]string         `yaml:"api_keys"`
    Providers    map[string]ProviderConfig `yaml:"providers"`
    Memory       MemoryConfig              `yaml:"memory"`
    Tools        ToolsConfig               `yaml:"tools"`
}

type ProviderConfig struct {
    BaseURL string            `yaml:"base_url"`
    Headers map[string]string `yaml:"headers"`
}

type ToolsConfig struct {
    Builtin    []string                    `yaml:"builtin"`
    MCPServers map[string]MCPServerConfig  `yaml:"mcp_servers"`
}

type MCPServerConfig struct {
    Transport string            `yaml:"transport"`
    Command   string            `yaml:"command"`
    Args      []string          `yaml:"args"`
    Env       map[string]string `yaml:"env"`
    URL       string            `yaml:"url"`
    Headers   map[string]string `yaml:"headers"`
}
```

Keep existing `MemoryConfig`.

- [ ] **Step 2: Add env var expansion to config loading** (`internal/config/config.go`)

Add `ApplyEnvVars(cfg *cobot.Config)` function that reads:
- `COBOT_MODEL` → cfg.Model
- `COBOT_WORKSPACE` → cfg.Workspace
- `OPENAI_API_KEY` → cfg.APIKeys["openai"]
- `ANTHROPIC_API_KEY` → cfg.APIKeys["anthropic"]

```go
func ApplyEnvVars(cfg *cobot.Config) {
    if v := os.Getenv("COBOT_MODEL"); v != "" {
        cfg.Model = v
    }
    if v := os.Getenv("COBOT_WORKSPACE"); v != "" {
        cfg.Workspace = v
    }
    if v := os.Getenv("OPENAI_API_KEY"); v != "" {
        if cfg.APIKeys == nil {
            cfg.APIKeys = make(map[string]string)
        }
        cfg.APIKeys["openai"] = v
    }
    if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
        if cfg.APIKeys == nil {
            cfg.APIKeys = make(map[string]string)
        }
        cfg.APIKeys["anthropic"] = v
    }
}
```

- [ ] **Step 3: Add workspace config loading** (`internal/config/config.go`)

Add `LoadWorkspaceConfig(cfg *cobot.Config, workspaceDir string) error` that looks for `.cobot/config.yaml` and overlays it.

```go
func LoadWorkspaceConfig(cfg *cobot.Config, workspaceDir string) error {
    wsConfig := filepath.Join(workspaceDir, ".cobot", "config.yaml")
    if _, err := os.Stat(wsConfig); err != nil {
        return nil
    }
    return LoadFromFile(cfg, wsConfig)
}
```

- [ ] **Step 4: Update `cmd/cobot/root.go` loadConfig()** to apply layering in order:

```go
func loadConfig() (*cobot.Config, error) {
    if err := workspace.EnsureGlobalDirs(); err != nil {
        fmt.Fprintf(os.Stderr, "warning: %v\n", err)
    }

    cfg := cobot.DefaultConfig()

    // Layer 4: Global config
    if cfgPath != "" {
        if _, err := os.Stat(cfgPath); err == nil {
            if err := config.LoadFromFile(cfg, cfgPath); err != nil {
                return nil, fmt.Errorf("load config: %w", err)
            }
        }
    } else {
        globalCfg := xdg.GlobalConfigPath()
        if _, err := os.Stat(globalCfg); err == nil {
            if err := config.LoadFromFile(cfg, globalCfg); err != nil {
                return nil, fmt.Errorf("load global config: %w", err)
            }
        }
    }

    // Layer 3: Workspace config
    if workspacePath != "" {
        cfg.Workspace = workspacePath
        if err := config.LoadWorkspaceConfig(cfg, workspacePath); err != nil {
            fmt.Fprintf(os.Stderr, "warning: %v\n", err)
        }
    } else if ws, err := workspace.Discover(); err == nil {
        cfg.Workspace = ws
        if err := config.LoadWorkspaceConfig(cfg, ws); err != nil {
            fmt.Fprintf(os.Stderr, "warning: %v\n", err)
        }
    }

    // Layer 2: Environment variables
    config.ApplyEnvVars(cfg)

    // Layer 1: CLI flags
    if modelName != "" {
        cfg.Model = modelName
    }

    return cfg, nil
}
```

- [ ] **Step 5: Add tests** (`internal/config/config_test.go`)

```go
func TestApplyEnvVars(t *testing.T) {
    t.Setenv("COBOT_MODEL", "test-model")
    t.Setenv("OPENAI_API_KEY", "sk-test123")

    cfg := cobot.DefaultConfig()
    ApplyEnvVars(cfg)

    if cfg.Model != "test-model" {
        t.Errorf("expected test-model, got %s", cfg.Model)
    }
    if cfg.APIKeys["openai"] != "sk-test123" {
        t.Error("expected openai API key")
    }
}

func TestLoadWorkspaceConfig(t *testing.T) {
    dir := t.TempDir()
    wsDir := filepath.Join(dir, ".cobot")
    os.MkdirAll(wsDir, 0755)

    yamlData := "model: workspace-model\nmax_turns: 99\n"
    os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte(yamlData), 0644)

    cfg := cobot.DefaultConfig()
    err := LoadWorkspaceConfig(cfg, dir)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Model != "workspace-model" {
        t.Errorf("expected workspace-model, got %s", cfg.Model)
    }
    if cfg.MaxTurns != 99 {
        t.Errorf("expected 99, got %d", cfg.MaxTurns)
    }
}

func TestLoadWorkspaceConfigMissing(t *testing.T) {
    dir := t.TempDir()
    cfg := cobot.DefaultConfig()
    err := LoadWorkspaceConfig(cfg, dir)
    if err != nil {
        t.Fatal("expected nil error for missing workspace config")
    }
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/config/... -v`

- [ ] **Step 7: Commit**

```bash
git add internal/config/ pkg/options.go cmd/cobot/root.go
git commit -m "feat: config layering with env vars and workspace config"
```

---

### Task 2: CLI — tools, config, memory Commands

**Files:**
- Create: `cmd/cobot/tools.go`
- Create: `cmd/cobot/config_cmd.go`
- Create: `cmd/cobot/memory_cmd.go`

**Goal:** Add `cobot tools`, `cobot config get/set`, `cobot memory search/status` CLI commands.

- [ ] **Step 1: Create `cmd/cobot/tools.go`**

```go
package main

import (
    "github.com/spf13/cobra"
    "github.com/cobot-agent/cobot/internal/tools/builtin"
)

var toolsCmd = &cobra.Command{
    Use:   "tools",
    Short: "List and manage tools",
}

var toolsListCmd = &cobra.Command{
    Use:   "list",
    Short: "List available tools",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        builtinTools := []string{
            "filesystem_read", "filesystem_write", "shell_exec",
            "memory_search", "memory_store", "subagent_spawn",
        }

        fmt.Fprintf(cmd.OutOrStdout(), "Built-in tools (%d):\n", len(builtinTools))
        for _, name := range builtinTools {
            fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
        }

        if cfg.Tools.MCPServers != nil {
            fmt.Fprintf(cmd.OutOrStdout(), "\nMCP servers (%d):\n", len(cfg.Tools.MCPServers))
            for name, srv := range cfg.Tools.MCPServers {
                fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", name, srv.Transport)
            }
        }

        return nil
    },
}

func init() {
    toolsCmd.AddCommand(toolsListCmd)
    rootCmd.AddCommand(toolsCmd)
}
```

Add `"fmt"` to imports.

- [ ] **Step 2: Create `cmd/cobot/config_cmd.go`**

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
    Use:   "config",
    Short: "View and manage configuration",
}

var configGetCmd = &cobra.Command{
    Use:   "get [key]",
    Short: "Get a config value",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }
        data, _ := json.MarshalIndent(cfg, "", "  ")
        fmt.Fprintln(cmd.OutOrStdout(), string(data))
        return nil
    },
}

var configShowCmd = &cobra.Command{
    Use:   "show",
    Short: "Show full resolved config",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }
        masked := *cfg
        if masked.APIKeys != nil {
            masked.APIKeys = make(map[string]string)
            for k := range cfg.APIKeys {
                masked.APIKeys[k] = "***"
            }
        }
        data, _ := json.MarshalIndent(masked, "", "  ")
        fmt.Fprintln(cmd.OutOrStdout(), string(data))
        return nil
    },
}

func init() {
    configCmd.AddCommand(configGetCmd)
    configCmd.AddCommand(configShowCmd)
    rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 3: Create `cmd/cobot/memory_cmd.go`**

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "path/filepath"

    "github.com/spf13/cobra"

    "github.com/cobot-agent/cobot/internal/memory"
    "github.com/cobot-agent/cobot/internal/xdg"
    cobot "github.com/cobot-agent/cobot/pkg"
)

var memoryCmd = &cobra.Command{
    Use:   "memory",
    Short: "Search and inspect memory palace",
}

var memorySearchCmd = &cobra.Command{
    Use:   "search [query]",
    Short: "Search memory",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        dataDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
        store, err := memory.OpenStore(dataDir)
        if err != nil {
            return err
        }
        defer store.Close()

        wingID, _ := cmd.Flags().GetString("wing")
        results, err := store.Search(context.Background(), &cobot.SearchQuery{
            Text:   args[0],
            WingID: wingID,
            Limit:  10,
        })
        if err != nil {
            return err
        }

        if len(results) == 0 {
            fmt.Fprintln(cmd.OutOrStdout(), "No results found.")
            return nil
        }

        for _, r := range results {
            fmt.Fprintf(cmd.OutOrStdout(), "[%s] %.2f %s\n", r.DrawerID, r.Score, truncate(r.Content, 120))
        }
        return nil
    },
}

var memoryStatusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show memory palace overview",
    RunE: func(cmd *cobra.Command, args []string) error {
        dataDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
        store, err := memory.OpenStore(dataDir)
        if err != nil {
            return err
        }
        defer store.Close()

        wings, err := store.GetWings(context.Background())
        if err != nil {
            return err
        }

        fmt.Fprintf(cmd.OutOrStdout(), "Memory Palace: %d wings\n", len(wings))
        for _, w := range wings {
            rooms, _ := store.GetRooms(context.Background(), w.ID)
            fmt.Fprintf(cmd.OutOrStdout(), "  Wing: %s (%s) — %d rooms\n", w.Name, w.ID, len(rooms))
            for _, r := range rooms {
                fmt.Fprintf(cmd.OutOrStdout(), "    Room: %s [%s]\n", r.Name, r.HallType)
            }
        }
        return nil
    },
}

func init() {
    memorySearchCmd.Flags().StringP("wing", "w", "", "Filter by wing ID")
    memoryCmd.AddCommand(memorySearchCmd)
    memoryCmd.AddCommand(memoryStatusCmd)
    rootCmd.AddCommand(memoryCmd)
}
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/cobot/...`

- [ ] **Step 5: Commit**

```bash
git add cmd/cobot/tools.go cmd/cobot/config_cmd.go cmd/cobot/memory_cmd.go
git commit -m "feat: add tools list, config show, memory search/status CLI commands"
```

---

### Task 3: SDK Enhancement — pkg/ Public API

**Files:**
- Modify: `pkg/cobot.go`
- Modify: `pkg/cobot_test.go`

**Goal:** Expose memory, ACP, subagent capabilities through the SDK public API.

- [ ] **Step 1: Expand `AgentCore` interface** (`pkg/cobot.go`)

```go
type AgentCore interface {
    SetProvider(Provider)
    SetMemoryStore(MemoryStore)
    Prompt(ctx context.Context, message string) (*ProviderResponse, error)
    Stream(ctx context.Context, message string) (<-chan Event, error)
    RegisterTool(Tool)
    ToolRegistry() ToolRegistrar
    MemoryStore() MemoryStore
    Config() *Config
    Close() error
}

type ToolRegistrar interface {
    Register(tool Tool)
    Get(name string) (Tool, error)
    ToolDefs() []ToolDef
}
```

Wait — `ToolRegistrar` is a new interface, and `MemoryStore` is already in `pkg/interfaces.go`. But `AgentCore` needs to include the new methods that `internal/agent.Agent` satisfies. Let me check what `internal/agent.Agent` actually has:

The `internal/agent.Agent` has: `SetProvider`, `SetMemoryStore`, `Prompt`, `Stream`, `RegisterTool`, `ToolRegistry`, `MemoryStore`, `Config`, `Close`, `Provider`, `Session`, `SetToolRegistry`. All the methods we need.

- [ ] **Step 2: Update Agent struct and methods** (`pkg/cobot.go`)

```go
package cobot

import "context"

type AgentCore interface {
    SetProvider(Provider)
    SetMemoryStore(MemoryStore)
    Prompt(ctx context.Context, message string) (*ProviderResponse, error)
    Stream(ctx context.Context, message string) (<-chan Event, error)
    RegisterTool(Tool)
    Close() error
    MemoryStore() MemoryStore
    Config() *Config
}

type Agent struct {
    core   AgentCore
    config *Config
}

func New(config *Config, core AgentCore) (*Agent, error) {
    if config == nil {
        config = DefaultConfig()
    }
    return &Agent{
        core:   core,
        config: config,
    }, nil
}

func (a *Agent) SetProvider(p Provider)          { a.core.SetProvider(p) }
func (a *Agent) SetMemoryStore(s MemoryStore)     { a.core.SetMemoryStore(s) }
func (a *Agent) MemoryStore() MemoryStore         { return a.core.MemoryStore() }
func (a *Agent) Config() *Config                  { return a.core.Config() }

func (a *Agent) Prompt(ctx context.Context, message string) (*ProviderResponse, error) {
    return a.core.Prompt(ctx, message)
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan Event, error) {
    return a.core.Stream(ctx, message)
}

func (a *Agent) RegisterTool(tool Tool) error {
    a.core.RegisterTool(tool)
    return nil
}

func (a *Agent) Close() error {
    return a.core.Close()
}
```

- [ ] **Step 3: Update `pkg/cobot_test.go`**

Need a mock that satisfies the expanded `AgentCore`. Read current test, update mock to include `SetMemoryStore`, `MemoryStore`, `Config`.

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/... -v`

- [ ] **Step 5: Commit**

```bash
git add pkg/
git commit -m "feat: enhance SDK with memory store and config accessors"
```

---

### Task 4: Bubbletea TUI

**Files:**
- Create: `cmd/cobot/tui.go`

**Goal:** When `cobot` is run without arguments, start a Bubbletea TUI for interactive chat. Add `go get` for bubbletea deps.

- [ ] **Step 1: Add Bubbletea dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/glamour@latest
```

If `charm.land` v2 URLs are used in go.mod, use those instead. Check what the design spec says: `charm.land/bubbletea/v2`. Try both and use whichever resolves. If neither resolves as `charm.land`, use the `github.com/charmbracelet` versions which are the standard ones.

- [ ] **Step 2: Create `cmd/cobot/tui.go`**

A minimal but functional TUI with:
- Text input for user messages
- Streaming response display
- Ctrl+C to interrupt
- `/quit` or `q` to exit

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/lipgloss"

    "github.com/cobot-agent/cobot/internal/agent"
    "github.com/cobot-agent/cobot/internal/llm/openai"
    "github.com/cobot-agent/cobot/internal/memory"
    "github.com/cobot-agent/cobot/internal/tools/builtin"
    "github.com/cobot-agent/cobot/internal/xdg"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type tuiModel struct {
    input    textinput.Model
    messages []string
    agent    *agent.Agent
    streaming bool
    cancelFn context.CancelFunc
    err      error
    width    int
    height   int
}

type streamChunkMsg struct {
    content string
    done    bool
    err     error
}

func newTUIModel(a *agent.Agent) tuiModel {
    ti := textinput.New()
    ti.Placeholder = "Type a message..."
    ti.Focus()
    ti.CharLimit = 4096

    return tuiModel{
        input:    ti,
        agent:    a,
        messages: []string{},
    }
}

func (m tuiModel) Init() tea.Cmd {
    return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case tea.KeyMsg:
        switch msg.Type {
        case tea.KeyCtrlC:
            if m.streaming && m.cancelFn != nil {
                m.cancelFn()
                m.streaming = false
                return m, nil
            }
            return m, tea.Quit
        case tea.KeyEnter:
            text := strings.TrimSpace(m.input.Value())
            if text == "" {
                return m, nil
            }
            if text == "/quit" || text == "q" {
                return m, tea.Quit
            }
            m.messages = append(m.messages, fmt.Sprintf("You: %s", text))
            m.input.SetValue("")
            m.streaming = true
            return m, m.startStream(text)
        }

    case streamChunkMsg:
        if msg.err != nil {
            m.streaming = false
            m.messages = append(m.messages, fmt.Sprintf("Error: %v", msg.err))
            return m, nil
        }
        if msg.done {
            m.streaming = false
            m.messages = append(m.messages, "")
            return m, nil
        }
        if len(m.messages) > 0 && !m.streaming {
            m.messages = append(m.messages, "Assistant: ")
        }
        if len(m.messages) > 0 {
            m.messages[len(m.messages)-1] += msg.content
        }
        return m, nil
    }

    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)
    return m, cmd
}

func (m tuiModel) View() string {
    var b strings.Builder

    for _, msg := range m.messages {
        b.WriteString(msg)
        b.WriteString("\n")
    }

    if m.streaming {
        b.WriteString(lipgloss.NewStyle().Faint(true).Render("Thinking..."))
        b.WriteString("\n")
    }

    b.WriteString("\n")
    b.WriteString(m.input.View())

    return b.String()
}

func (m *tuiModel) startStream(text string) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithCancel(context.Background())
        m.cancelFn = cancel

        ch, err := m.agent.Stream(ctx, text)
        if err != nil {
            return streamChunkMsg{err: err}
        }

        var accumulated string
        for chunk := range ch {
            if chunk.Type == cobot.EventText {
                accumulated += chunk.Content
            }
            if chunk.Type == cobot.EventError {
                return streamChunkMsg{err: chunk.Error}
            }
            if chunk.Type == cobot.EventDone {
                return streamChunkMsg{content: accumulated, done: true}
            }
        }

        return streamChunkMsg{content: accumulated, done: true}
    }
}

var tuiCmd = &cobra.Command{
    Use:   "tui",
    Short: "Start interactive TUI",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        a := agent.New(cfg)

        apiKey := cfg.APIKeys["openai"]
        if apiKey != "" {
            a.SetProvider(openai.NewProvider(apiKey, ""))
        }

        memDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
        if ms, err := memory.OpenStore(memDir); err == nil {
            a.SetMemoryStore(ms)
        }

        a.RegisterTool(builtin.NewReadFileTool())
        a.RegisterTool(builtin.NewWriteFileTool())
        a.RegisterTool(builtin.NewShellExecTool())

        p := tea.NewProgram(newTUIModel(a), tea.WithAltScreen())
        _, err = p.Run()
        return err
    },
}

func init() {
    rootCmd.AddCommand(tuiCmd)
}
```

Wait — `bubbletea`, `textinput`, `lipgloss` import paths vary. After `go get`, check the actual module paths. The `github.com/charmbracelet/bubbletea` package uses `tea` not `bubbletea`. Let me fix:

```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/lipgloss"
)
```

- [ ] **Step 3: Make `cobot` with no args start TUI** — Update `root.go` to run TUI when no subcommand:

Add `RunE` to rootCmd that delegates to TUI:

```go
var rootCmd = &cobra.Command{
    Use:     "cobot",
    Short:   "A personal AI agent system",
    Long:    "Cobot is a Go-based personal agent system with memory, tools, and protocols.",
    Version: "0.1.0",
    RunE: func(cmd *cobra.Command, args []string) error {
        return tuiCmd.RunE(cmd, args)
    },
}
```

- [ ] **Step 4: Build and verify**

Run: `go mod tidy && go build ./cmd/cobot/...`

- [ ] **Step 5: Commit**

```bash
git add cmd/cobot/tui.go cmd/cobot/root.go go.mod go.sum
git commit -m "feat: add Bubbletea TUI for interactive chat mode"
```

---

### Task 5: Wire Chat Command with Full Stack

**Files:**
- Modify: `cmd/cobot/chat.go`

**Goal:** Update `cobot chat` to register all tools + memory, matching the TUI wiring.

- [ ] **Step 1: Update `cmd/cobot/chat.go`**

Replace current implementation with full wiring:

```go
var chatCmd = &cobra.Command{
    Use:   "chat [message]",
    Short: "Send a message to the agent",
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        core := agent.New(cfg)
        agt, err := cobot.New(cfg, core)
        if err != nil {
            return err
        }
        defer agt.Close()

        agt.RegisterTool(builtin.NewReadFileTool())
        agt.RegisterTool(builtin.NewWriteFileTool())
        agt.RegisterTool(builtin.NewShellExecTool())

        apiKey := cfg.APIKeys["openai"]
        if apiKey == "" {
            return fmt.Errorf("openai API key not configured (set api_keys.openai in config or OPENAI_API_KEY env)")
        }

        provider := openai.NewProvider(apiKey, "")
        agt.SetProvider(provider)

        memDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
        if ms, err := memory.OpenStore(memDir); err == nil {
            agt.SetMemoryStore(ms)
        }

        ch, err := agt.Stream(context.Background(), args[0])
        if err != nil {
            return err
        }

        for event := range ch {
            switch event.Type {
            case cobot.EventText:
                fmt.Print(event.Content)
            case cobot.EventToolCall:
                fmt.Fprintf(os.Stderr, "[Tool: %s]\n", event.ToolCall.Name)
            case cobot.EventToolResult:
                fmt.Fprintf(os.Stderr, "[Result: %s]\n", truncate(event.Content, 100))
            case cobot.EventDone:
                fmt.Println()
            case cobot.EventError:
                fmt.Fprintf(os.Stderr, "Error: %v\n", event.Error)
            }
        }
        return nil
    },
}
```

Add imports: `"path/filepath"`, `"github.com/cobot-agent/cobot/internal/memory"`, `"github.com/cobot-agent/cobot/internal/xdg"`.

- [ ] **Step 2: Build**

Run: `go build ./cmd/cobot/...`

- [ ] **Step 3: Commit**

```bash
git add cmd/cobot/chat.go
git commit -m "feat: wire memory store into chat command"
```

---

### Task 6: Final Verification

- [ ] **Step 1:** `go mod tidy`
- [ ] **Step 2:** `go build ./...`
- [ ] **Step 3:** `go vet ./...`
- [ ] **Step 4:** `go test ./... -count=1`
- [ ] **Step 5:** Verify new packages have tests passing
- [ ] **Step 6:** Fix any issues
- [ ] **Step 7:** Final commit if needed
