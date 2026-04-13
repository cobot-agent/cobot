# Phase 1: Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the core foundation of cobot — project structure, config system, LLM provider plugin architecture, agent loop, and basic built-in tools — enough to run a minimal `cobot chat "hello"` end-to-end.

**Architecture:** Plugin-based LLM providers behind a common interface, a think→act→observe agent loop with tool execution, layered config from flags/env/file/defaults, and directory-scoped workspace discovery.

**Tech Stack:** Go 1.26, cobra (CLI), spf13/pflag (flags), gopkg.in/yaml.v3 (config), net/http (LLM API calls)

**Design Spec:** `docs/specs/2026-04-12-cobot-agent-system-design.md`

---

## File Structure

```
cobot/
├── go.mod
├── go.sum
├── cmd/
│   └── cobot/
│       └── main.go                    # CLI entry point
├── internal/
│   ├── config/
│   │   ├── config.go                  # Config struct, LoadConfig, layering
│   │   └── defaults.go               # Default values
│   ├── llm/
│   │   ├── provider.go               # Provider interface, types
│   │   ├── registry.go               # Provider registry
│   │   └── openai/
│   │       ├── provider.go           # OpenAI-compatible provider
│   │       ├── stream.go             # SSE streaming parser
│   │       └── types.go              # OpenAI API types
│   ├── agent/
│   │   ├── agent.go                  # Agent struct, New, Prompt, Stream, Close
│   │   ├── loop.go                   # Think → Act → Observe cycle
│   │   ├── context.go                # Context building (system prompt, history)
│   │   └── session.go               # Session management
│   ├── tools/
│   │   ├── registry.go               # Tool interface, Registry
│   │   ├── executor.go               # Parallel tool execution
│   │   └── builtin/
│   │       ├── filesystem.go         # File read/write/search tools
│   │       └── shell.go              # Shell exec tool
│   └── workspace/
│       ├── workspace.go              # Workspace struct
│       └── discovery.go              # Find .cobot/ up the tree
├── pkg/
│   ├── cobot.go                      # Public SDK entry: New(config)
│   ├── types.go                      # Public types: Message, ToolCall, etc.
│   ├── interfaces.go                 # Public interfaces: Provider, Tool
│   ├── options.go                    # Config struct (public)
│   └── errors.go                     # Public error types
├── docs/
│   ├── specs/
│   │   └── 2026-04-12-cobot-agent-system-design.md
│   └── superpowers/
│       └── plans/
│           └── 2026-04-12-phase1-foundation.md
└── .cobot/
    ├── config.yaml                   # cobot's own workspace config (dogfooding)
    └── AGENTS.md                     # cobot's own personality
```

---

### Task 1: Project Structure and go.mod

**Files:**
- Create: `go.mod` (update existing)
- Create: `cmd/cobot/main.go`
- Create: `pkg/types.go`
- Create: `pkg/errors.go`

- [ ] **Step 1: Initialize go.mod with dependencies**

```bash
cd /Users/muk/Work/github.com/cobot-agent/cobot
go mod init github.com/cobot-agent/cobot
go get github.com/spf13/cobra@latest
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 2: Create pkg/types.go with core public types**

```go
package cobot

import (
	"encoding/json"
	"time"
)

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

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type StopReason string

const (
	StopEndTurn         StopReason = "end_turn"
	StopMaxTokens       StopReason = "max_tokens"
	StopMaxTurnRequests StopReason = "max_turn_requests"
	StopCancelled       StopReason = "cancelled"
	StopRefusal         StopReason = "refusal"
)

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ProviderRequest struct {
	Model       string     `json:"model"`
	Messages    []Message  `json:"messages"`
	Tools       []ToolDef  `json:"tools,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
	Temperature float64    `json:"temperature,omitempty"`
}

type ProviderResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason StopReason `json:"stop_reason"`
	Usage      Usage      `json:"usage"`
}

type ProviderChunk struct {
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Done     bool      `json:"done"`
	Usage    *Usage    `json:"usage,omitempty"`
}

type Event struct {
	Type     EventType
	Content  string
	ToolCall *ToolCall
	Done     bool
	Error    error
}

type EventType int

const (
	EventText EventType = iota
	EventToolCall
	EventToolResult
	EventDone
	EventError
)
```

- [ ] **Step 3: Create pkg/errors.go**

```go
package cobot

type CobotError struct {
	Code    string
	Message string
	Cause   error
}

func (e *CobotError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *CobotError) Unwrap() error { return e.Cause }

var (
	ErrProviderNotConfigured = &CobotError{Code: "PROVIDER_NOT_CONFIGURED", Message: "LLM provider not configured"}
	ErrWorkspaceNotFound     = &CobotError{Code: "WORKSPACE_NOT_FOUND", Message: "workspace not found"}
	ErrToolNotFound          = &CobotError{Code: "TOOL_NOT_FOUND", Message: "tool not found"}
	ErrMemorySearchFailed    = &CobotError{Code: "MEMORY_SEARCH_FAILED", Message: "memory search failed"}
	ErrMaxTurnsExceeded      = &CobotError{Code: "MAX_TURNS_EXCEEDED", Message: "max turns exceeded"}
	ErrAgentCancelled        = &CobotError{Code: "AGENT_CANCELLED", Message: "agent cancelled"}
)
```

- [ ] **Step 4: Create minimal cmd/cobot/main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("cobot v0.1.0")
	os.Exit(0)
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./cmd/cobot/`
Expected: builds without errors

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/ pkg/
git commit -m "feat: initialize project structure with core types and errors"
```

---

### Task 2: Public Interfaces and SDK Entry Point

**Files:**
- Create: `pkg/interfaces.go`
- Create: `pkg/options.go`
- Create: `pkg/cobot.go`

- [ ] **Step 1: Create pkg/interfaces.go with Provider and Tool interfaces**

```go
package cobot

import (
	"context"
	"encoding/json"
)

type Provider interface {
	Name() string
	Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)
}

type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

- [ ] **Step 2: Create pkg/options.go with public Config struct**

```go
package cobot

type Config struct {
	ConfigPath   string
	Workspace    string
	Model        string
	MaxTurns     int
	SystemPrompt string
	Verbose      bool
	APIKeys      map[string]string
}

type MemoryConfig struct {
	Enabled    bool
	BadgerPath string
	BlevePath  string
}

func DefaultConfig() *Config {
	return &Config{
		MaxTurns: 50,
		Model:    "openai:gpt-4o",
		APIKeys:  make(map[string]string),
	}
}
```

- [ ] **Step 3: Create pkg/cobot.go — SDK entry point (skeleton)**

```go
package cobot

import "context"

type Agent struct {
	config   *Config
	provider Provider
}

func New(config *Config) (*Agent, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &Agent{
		config: config,
	}, nil
}

func (a *Agent) Prompt(ctx context.Context, message string) (*ProviderResponse, error) {
	return nil, nil
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan Event, error) {
	return nil, nil
}

func (a *Agent) RegisterTool(tool Tool) error {
	return nil
}

func (a *Agent) Close() error {
	return nil
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: builds without errors

- [ ] **Step 5: Commit**

```bash
git add pkg/interfaces.go pkg/options.go pkg/cobot.go
git commit -m "feat: add public SDK interfaces and entry point skeleton"
```

---

### Task 3: Config System with Layered Loading

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/defaults.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for config loading**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxTurns != 50 {
		t.Errorf("expected MaxTurns=50, got %d", cfg.MaxTurns)
	}
	if cfg.Model != "openai:gpt-4o" {
		t.Errorf("expected Model=openai:gpt-4o, got %s", cfg.Model)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("model: anthropic:claude-3.5-sonnet\nmax_turns: 30\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := LoadFromFile(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "anthropic:claude-3.5-sonnet" {
		t.Errorf("expected anthropic:claude-3.5-sonnet, got %s", cfg.Model)
	}
	if cfg.MaxTurns != 30 {
		t.Errorf("expected MaxTurns=30, got %d", cfg.MaxTurns)
	}
}

func TestFlagOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("model: anthropic:claude-3.5-sonnet\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := LoadFromFile(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	cfg.Model = "openai:gpt-4o-mini"
	if cfg.Model != "openai:gpt-4o-mini" {
		t.Errorf("expected flag override openai:gpt-4o-mini, got %s", cfg.Model)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	os.Setenv("TEST_API_KEY", "sk-test-123")
	defer os.Unsetenv("TEST_API_KEY")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("api_keys:\n  openai: ${TEST_API_KEY}\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := LoadFromFile(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	if cfg.APIKeys["openai"] != "sk-test-123" {
		t.Errorf("expected expanded env var, got %s", cfg.APIKeys["openai"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: compile error — package does not exist

- [ ] **Step 3: Create internal/config/defaults.go**

```go
package config

import (
	cobot "github.com/cobot-agent/cobot/pkg"
)

func DefaultConfig() *cobot.Config {
	return cobot.DefaultConfig()
}
```

- [ ] **Step 4: Create internal/config/config.go**

```go
package config

import (
	"os"
	"regexp"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
	"gopkg.in/yaml.v3"
)

var envVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

func LoadFromFile(cfg *cobot.Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	expanded := expandEnvVars(string(data))
	return yaml.Unmarshal([]byte(expanded), cfg)
}

func expandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "${}")
		return os.Getenv(varName)
	})
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: all 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/
git commit -m "feat: add config system with YAML loading and env var expansion"
```

---

### Task 4: LLM Provider Interface and Registry

**Files:**
- Create: `internal/llm/provider.go`
- Create: `internal/llm/registry.go`
- Test: `internal/llm/registry_test.go`

- [ ] **Step 1: Write failing test for provider registry**

```go
package llm

import (
	"context"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	return &cobot.ProviderResponse{Content: "mock response", StopReason: cobot.StopEndTurn}, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	ch := make(chan cobot.ProviderChunk, 1)
	ch <- cobot.ProviderChunk{Content: "mock", Done: true}
	close(ch)
	return ch, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "openai"}
	r.Register("openai", p)

	got, err := r.Get("openai")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != "openai" {
		t.Errorf("expected openai, got %s", got.Name())
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register("openai", &mockProvider{name: "openai"})
	r.Register("anthropic", &mockProvider{name: "anthropic"})

	names := r.List()
	if len(names) != 2 {
		t.Errorf("expected 2 providers, got %d", len(names))
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/llm/ -v`
Expected: compile error

- [ ] **Step 3: Create internal/llm/registry.go**

```go
package llm

import (
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type Registry struct {
	providers map[string]cobot.Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]cobot.Provider),
	}
}

func (r *Registry) Register(name string, p cobot.Provider) {
	r.providers[name] = p
}

func (r *Registry) Get(name string) (cobot.Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/llm/ -v`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/llm/
git commit -m "feat: add LLM provider interface and registry"
```

---

### Task 5: OpenAI-Compatible Provider

**Files:**
- Create: `internal/llm/openai/types.go`
- Create: `internal/llm/openai/provider.go`
- Create: `internal/llm/openai/stream.go`
- Test: `internal/llm/openai/provider_test.go`

- [ ] **Step 1: Create internal/llm/openai/types.go — OpenAI API request/response types**

```go
package openai

import (
	cobot "github.com/cobot-agent/cobot/pkg"
)

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []chatTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content"`
	ToolCalls  []chatToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFunctionCall `json:"function"`
}

type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamChunk struct {
	ID      string           `json:"id"`
	Choices []streamChoice   `json:"choices"`
}

type streamChoice struct {
	Index        int            `json:"index"`
	Delta        streamDelta    `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

type streamDelta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []chatToolCall  `json:"tool_calls,omitempty"`
}

func toProviderResponse(resp *chatResponse) *cobot.ProviderResponse {
	pr := &cobot.ProviderResponse{
		StopReason: cobot.StopEndTurn,
		Usage: cobot.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		pr.Content = choice.Message.Content
		for _, tc := range choice.Message.ToolCalls {
			pr.ToolCalls = append(pr.ToolCalls, cobot.ToolCall{
				ID:       tc.ID,
				Name:     tc.Function.Name,
				Arguments: []byte(tc.Function.Arguments),
			})
		}
		switch choice.FinishReason {
		case "stop":
			pr.StopReason = cobot.StopEndTurn
		case "length":
			pr.StopReason = cobot.StopMaxTokens
		case "tool_calls":
			pr.StopReason = cobot.StopEndTurn
		case "content_filter":
			pr.StopReason = cobot.StopRefusal
		}
	}
	return pr
}

func fromProviderMessages(msgs []cobot.Message) []chatMessage {
	out := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		cm := chatMessage{Role: string(m.Role), Content: m.Content}
		for _, tc := range m.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, chatToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: chatFunctionCall{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}
		if m.ToolResult != nil {
			cm.ToolCallID = m.ToolResult.CallID
			cm.Content = m.ToolResult.Output
			if m.ToolResult.Error != "" {
				cm.Content = "Error: " + m.ToolResult.Error
			}
		}
		out[i] = cm
	}
	return out
}

func fromProviderTools(tools []cobot.ToolDef) []chatTool {
	out := make([]chatTool, len(tools))
	for i, t := range tools {
		out[i] = chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return out
}
```

- [ ] **Step 2: Create internal/llm/openai/provider.go — OpenAI provider implementation**

```go
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewProvider(apiKey, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{},
	}
}

func (p *Provider) Name() string { return "openai" }

func (p *Provider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	body := chatRequest{
		Model:       req.Model,
		Messages:    fromProviderMessages(req.Messages),
		Tools:       fromProviderTools(req.Tools),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return toProviderResponse(&chatResp), nil
}

func (p *Provider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	body := chatRequest{
		Model:       req.Model,
		Messages:    fromProviderMessages(req.Messages),
		Tools:       fromProviderTools(req.Tools),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan cobot.ProviderChunk, 64)
	go parseSSEStream(resp.Body, ch)
	return ch, nil
}

func parseSSEStream(body io.Reader, ch chan<- cobot.ProviderChunk) {
	defer close(ch)
	decoder := json.NewDecoder(body)
	for {
		var chunk streamChunk
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		pc := cobot.ProviderChunk{
			Content: choice.Delta.Content,
		}
		for _, tc := range choice.Delta.ToolCalls {
			pc.ToolCall = &cobot.ToolCall{
				ID:       tc.ID,
				Name:     tc.Function.Name,
				Arguments: []byte(tc.Function.Arguments),
			}
		}
		if choice.FinishReason != nil {
			pc.Done = true
		}
		ch <- pc
	}
	ch <- cobot.ProviderChunk{Done: true}
}
```

- [ ] **Step 3: Write test for OpenAI provider construction**

```go
package openai

import (
	"testing"
)

func TestNewProviderDefaultBaseURL(t *testing.T) {
	p := NewProvider("test-key", "")
	if p.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default baseURL, got %s", p.baseURL)
	}
}

func TestNewProviderCustomBaseURL(t *testing.T) {
	p := NewProvider("test-key", "https://openrouter.ai/api/v1")
	if p.baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected custom baseURL, got %s", p.baseURL)
	}
}

func TestNewProviderTrailingSlash(t *testing.T) {
	p := NewProvider("test-key", "https://api.example.com/v1/")
	if p.baseURL != "https://api.example.com/v1" {
		t.Errorf("expected trimmed baseURL, got %s", p.baseURL)
	}
}

func TestProviderName(t *testing.T) {
	p := NewProvider("key", "")
	if p.Name() != "openai" {
		t.Errorf("expected name openai, got %s", p.Name())
	}
}

func TestToProviderResponseToolCalls(t *testing.T) {
	fr := "tool_calls"
	resp := &chatResponse{
		Choices: []chatChoice{{
			Message: chatMessage{
				Role:    "assistant",
				Content: "",
				ToolCalls: []chatToolCall{{
					ID:   "call_123",
					Type: "function",
					Function: chatFunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"/tmp/test.txt"}`,
					},
				}},
			},
			FinishReason: fr,
		}},
	}
	pr := toProviderResponse(resp)
	if len(pr.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(pr.ToolCalls))
	}
	if pr.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected read_file, got %s", pr.ToolCalls[0].Name)
	}
}

func TestFromProviderMessages(t *testing.T) {
	msgs := []cobot.Message{
		{Role: cobot.RoleSystem, Content: "You are helpful."},
		{Role: cobot.RoleUser, Content: "Hello"},
	}
	out := fromProviderMessages(msgs)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if out[0].Role != "system" {
		t.Errorf("expected system role, got %s", out[0].Role)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/llm/openai/ -v`
Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/llm/openai/
git commit -m "feat: add OpenAI-compatible LLM provider with streaming"
```

---

### Task 6: Tool Interface and Registry

**Files:**
- Create: `internal/tools/registry.go`
- Create: `internal/tools/executor.go`
- Test: `internal/tools/registry_test.go`

- [ ] **Step 1: Write failing test for tool registry**

```go
package tools

import (
	"context"
	"encoding/json"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type mockTool struct {
	name string
}

func (m *mockTool) Name() string            { return m.name }
func (m *mockTool) Description() string     { return "mock tool" }
func (m *mockTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (m *mockTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return "mock result", nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "test_tool"}
	r.Register(tool)

	got, err := r.Get("test_tool")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != "test_tool" {
		t.Errorf("expected test_tool, got %s", got.Name())
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "tool_a"})
	r.Register(&mockTool{name: "tool_b"})

	defs := r.ToolDefs()
	if len(defs) != 2 {
		t.Errorf("expected 2 tools, got %d", len(defs))
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestExecuteTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "test_tool"})
	result, err := r.Execute(context.Background(), cobot.ToolCall{
		ID:       "call_1",
		Name:     "test_tool",
		Arguments: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "mock result" {
		t.Errorf("expected 'mock result', got %s", result.Output)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tools/ -v`
Expected: compile error

- [ ] **Step 3: Create internal/tools/registry.go**

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]cobot.Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]cobot.Tool),
	}
}

func (r *Registry) Register(t cobot.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (cobot.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return t, nil
}

func (r *Registry) ToolDefs() []cobot.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]cobot.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, cobot.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, call cobot.ToolCall) (*cobot.ToolResult, error) {
	t, err := r.Get(call.Name)
	if err != nil {
		return &cobot.ToolResult{
			CallID: call.ID,
			Error:  err.Error(),
		}, nil
	}
	output, err := t.Execute(ctx, call.Arguments)
	if err != nil {
		return &cobot.ToolResult{
			CallID: call.ID,
			Error:  err.Error(),
		}, nil
	}
	return &cobot.ToolResult{
		CallID: call.ID,
		Output: output,
	}, nil
}

func (r *Registry) ExecuteParallel(ctx context.Context, calls []cobot.ToolCall) []*cobot.ToolResult {
	results := make([]*cobot.ToolResult, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c cobot.ToolCall) {
			defer wg.Done()
			result, _ := r.Execute(ctx, c)
			results[idx] = result
		}(i, call)
	}
	wg.Wait()
	return results
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tools/ -v`
Expected: all 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tools/
git commit -m "feat: add tool registry with parallel execution"
```

---

### Task 7: Built-in Tools (Filesystem and Shell)

**Files:**
- Create: `internal/tools/builtin/filesystem.go`
- Create: `internal/tools/builtin/shell.go`
- Test: `internal/tools/builtin/filesystem_test.go`
- Test: `internal/tools/builtin/shell_test.go`

- [ ] **Step 1: Write failing test for filesystem tools**

```go
package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileTool(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello world"), 0644)

	tool := NewReadFileTool()
	args, _ := json.Marshal(map[string]string{"path": f})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %s", result)
	}
}

func TestWriteFileTool(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "output.txt")

	tool := NewWriteFileTool()
	args, _ := json.Marshal(map[string]string{"path": f, "content": "written content"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Errorf("expected ok, got %s", result)
	}
	data, _ := os.ReadFile(f)
	if string(data) != "written content" {
		t.Errorf("file content mismatch: %s", string(data))
	}
}

func TestReadFileNotFound(t *testing.T) {
	tool := NewReadFileTool()
	args, _ := json.Marshal(map[string]string{"path": "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tools/builtin/ -v -run TestRead`
Expected: compile error

- [ ] **Step 3: Create internal/tools/builtin/filesystem.go**

```go
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type readFileArgs struct {
	Path string `json:"path"`
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool { return &ReadFileTool{} }

func (t *ReadFileTool) Name() string        { return "filesystem_read" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file at the given path" }
func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {"path": {"type": "string", "description": "Absolute path to the file"}},
		"required": ["path"]
	}`)
}
func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool { return &WriteFileTool{} }

func (t *WriteFileTool) Name() string        { return "filesystem_write" }
func (t *WriteFileTool) Description() string { return "Write content to a file at the given path" }
func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Absolute path to the file"},
			"content": {"type": "string", "description": "Content to write"}
		},
		"required": ["path", "content"]
	}`)
}
func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if err := os.WriteFile(a.Path, []byte(a.Content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return "ok", nil
}
```

- [ ] **Step 4: Run filesystem tests**

Run: `go test ./internal/tools/builtin/ -v -run TestReadFile`
Expected: all PASS

- [ ] **Step 5: Write test for shell tool**

```go
func TestShellExecTool(t *testing.T) {
	tool := NewShellExecTool()
	args, _ := json.Marshal(map[string]string{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result)
	}
}

func TestShellExecToolMultiArg(t *testing.T) {
	tool := NewShellExecTool()
	args, _ := json.Marshal(map[string]string{"command": "echo hello world"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", result)
	}
}
```

- [ ] **Step 6: Create internal/tools/builtin/shell.go**

```go
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type shellExecArgs struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
}

type ShellExecTool struct{}

func NewShellExecTool() *ShellExecTool { return &ShellExecTool{} }

func (t *ShellExecTool) Name() string        { return "shell_exec" }
func (t *ShellExecTool) Description() string { return "Execute a shell command and return its output" }
func (t *ShellExecTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to execute"},
			"dir": {"type": "string", "description": "Working directory (optional)"}
		},
		"required": ["command"]
	}`)
}
func (t *ShellExecTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a shellExecArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", a.Command)
	if a.Dir != "" {
		cmd.Dir = a.Dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command failed: %w", err)
	}
	return string(out), nil
}
```

- [ ] **Step 7: Run all builtin tests**

Run: `go test ./internal/tools/builtin/ -v`
Expected: all 5 tests PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tools/builtin/
git commit -m "feat: add filesystem and shell built-in tools"
```

---

### Task 8: Workspace Discovery

**Files:**
- Create: `internal/workspace/workspace.go`
- Create: `internal/workspace/discovery.go`
- Test: `internal/workspace/discovery_test.go`

- [ ] **Step 1: Write failing test for workspace discovery**

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cobot"), 0755)
	os.WriteFile(filepath.Join(dir, ".cobot", "config.yaml"), []byte("model: openai:gpt-4o\n"), 0644)

	ws, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root != dir {
		t.Errorf("expected root %s, got %s", dir, ws.Root)
	}
	if ws.ConfigPath != filepath.Join(dir, ".cobot", "config.yaml") {
		t.Errorf("unexpected config path: %s", ws.ConfigPath)
	}
}

func TestDiscoverInParentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cobot"), 0755)
	os.WriteFile(filepath.Join(dir, ".cobot", "config.yaml"), []byte("model: openai:gpt-4o\n"), 0644)

	subdir := filepath.Join(dir, "sub", "project")
	os.MkdirAll(subdir, 0755)

	ws, err := Discover(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root != dir {
		t.Errorf("expected root %s, got %s", dir, ws.Root)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Discover(dir)
	if err == nil {
		t.Error("expected error when no .cobot found")
	}
}

func TestInitWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root != dir {
		t.Errorf("expected root %s, got %s", dir, ws.Root)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cobot", "config.yaml")); os.IsNotExist(err) {
		t.Error("config.yaml not created")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/workspace/ -v`
Expected: compile error

- [ ] **Step 3: Create internal/workspace/workspace.go**

```go
package workspace

import "path/filepath"

type Workspace struct {
	Root       string
	ConfigPath string
	AgentsMd   string
}

func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.Root, ".cobot", "memory")
}

func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.Root, ".cobot", "sessions")
}

func (w *Workspace) ToolsConfigPath() string {
	return filepath.Join(w.Root, ".cobot", "tools.yaml")
}

func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.Root, ".cobot", "skills")
}
```

- [ ] **Step 4: Create internal/workspace/discovery.go**

```go
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

func Discover(startDir string) (*Workspace, error) {
	dir := startDir
	for {
		cobotDir := filepath.Join(dir, ".cobot")
		info, err := os.Stat(cobotDir)
		if err == nil && info.IsDir() {
			ws := &Workspace{
				Root:       dir,
				ConfigPath: filepath.Join(cobotDir, "config.yaml"),
			}
			agentsMd := filepath.Join(cobotDir, "AGENTS.md")
			if _, err := os.Stat(agentsMd); err == nil {
				ws.AgentsMd = agentsMd
			}
			return ws, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no .cobot directory found from %s", startDir)
		}
		dir = parent
	}
}

func Init(dir string) (*Workspace, error) {
	cobotDir := filepath.Join(dir, ".cobot")
	if err := os.MkdirAll(cobotDir, 0755); err != nil {
		return nil, fmt.Errorf("create .cobot: %w", err)
	}
	configPath := filepath.Join(cobotDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := []byte("model: openai:gpt-4o\nmax_turns: 50\n")
		if err := os.WriteFile(configPath, defaultConfig, 0644); err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}
	}
	agentsMd := filepath.Join(cobotDir, "AGENTS.md")
	if _, err := os.Stat(agentsMd); os.IsNotExist(err) {
		if err := os.WriteFile(agentsMd, []byte("# Cobot Agent\n\nYou are a helpful AI assistant.\n"), 0644); err != nil {
			return nil, fmt.Errorf("write AGENTS.md: %w", err)
		}
	}
	return &Workspace{
		Root:       dir,
		ConfigPath: configPath,
		AgentsMd:   agentsMd,
	}, nil
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/workspace/ -v`
Expected: all 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/
git commit -m "feat: add workspace discovery and initialization"
```

---

### Task 9: Agent Loop (Think → Act → Observe)

**Files:**
- Create: `internal/agent/agent.go`
- Create: `internal/agent/loop.go`
- Create: `internal/agent/context.go`
- Create: `internal/agent/session.go`
- Test: `internal/agent/loop_test.go`

- [ ] **Step 1: Write failing test for agent loop**

```go
package agent

import (
	"context"
	"encoding/json"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type mockProvider struct {
	responses []*cobot.ProviderResponse
	calls     int
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	if m.calls >= len(m.responses) {
		return &cobot.ProviderResponse{Content: "done", StopReason: cobot.StopEndTurn}, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	ch := make(chan cobot.ProviderChunk, 1)
	resp, err := m.Complete(ctx, req)
	if err != nil {
		close(ch)
		return ch, err
	}
	ch <- cobot.ProviderChunk{Content: resp.Content, Done: true}
	close(ch)
	return ch, nil
}

func TestAgentPromptSimpleResponse(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{Content: "Hello! How can I help?", StopReason: cobot.StopEndTurn},
		},
	})

	resp, err := a.Prompt(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello! How can I help?" {
		t.Errorf("unexpected response: %s", resp.Content)
	}
}

func TestAgentPromptToolCall(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{
				ToolCalls: []cobot.ToolCall{{
					ID:       "call_1",
					Name:     "echo",
					Arguments: json.RawMessage(`{"msg":"test"}`),
				}},
				StopReason: cobot.StopEndTurn,
			},
			{Content: "Echo result: test", StopReason: cobot.StopEndTurn},
		},
	})

	a.ToolRegistry().Register(&echoTool{})

	resp, err := a.Prompt(context.Background(), "echo test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Echo result: test" {
		t.Errorf("unexpected response: %s", resp.Content)
	}
}

func TestAgentMaxTurnsExceeded(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 2})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{ToolCalls: []cobot.ToolCall{{ID: "1", Name: "echo", Arguments: json.RawMessage(`{}`)}}, StopReason: cobot.StopEndTurn},
			{ToolCalls: []cobot.ToolCall{{ID: "2", Name: "echo", Arguments: json.RawMessage(`{}`)}}, StopReason: cobot.StopEndTurn},
			{ToolCalls: []cobot.ToolCall{{ID: "3", Name: "echo", Arguments: json.RawMessage(`{}`)}}, StopReason: cobot.StopEndTurn},
		},
	})
	a.ToolRegistry().Register(&echoTool{})

	_, err := a.Prompt(context.Background(), "loop")
	if err == nil {
		t.Error("expected error for max turns exceeded")
	}
}

type echoTool struct{}

func (e *echoTool) Name() string            { return "echo" }
func (e *echoTool) Description() string     { return "echo back input" }
func (e *echoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`)
}
func (e *echoTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct{ Msg string `json:"msg"` }
	json.Unmarshal(args, &a)
	return a.Msg, nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -v`
Expected: compile error

- [ ] **Step 3: Create internal/agent/agent.go**

```go
package agent

import (
	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/internal/tools"
)

type Agent struct {
	config   *cobot.Config
	provider cobot.Provider
	tools    *tools.Registry
	session  *Session
}

func New(config *cobot.Config) *Agent {
	return &Agent{
		config:  config,
		tools:   tools.NewRegistry(),
		session: NewSession(),
	}
}

func (a *Agent) SetProvider(p cobot.Provider) {
	a.provider = p
}

func (a *Agent) ToolRegistry() *tools.Registry {
	return a.tools
}

func (a *Agent) Session() *Session {
	return a.session
}

func (a *Agent) Close() error {
	return nil
}
```

- [ ] **Step 4: Create internal/agent/session.go**

```go
package agent

import (
	"sync"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type Session struct {
	mu       sync.RWMutex
	messages []cobot.Message
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Messages() []cobot.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cobot.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

func (s *Session) AddMessage(m cobot.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}
```

- [ ] **Step 5: Create internal/agent/context.go**

```go
package agent

import (
	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) buildMessages(userMessage string) []cobot.Message {
	msgs := a.session.Messages()
	msgs = append(msgs, cobot.Message{
		Role:    cobot.RoleUser,
		Content: userMessage,
	})
	return msgs
}
```

- [ ] **Step 6: Create internal/agent/loop.go**

```go
package agent

import (
	"context"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) Prompt(ctx context.Context, message string) (*cobot.ProviderResponse, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}

	a.session.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

	for turn := 0; turn < a.config.MaxTurns; turn++ {
		msgs := a.session.Messages()
		req := &cobot.ProviderRequest{
			Model:    a.config.Model,
			Messages: msgs,
			Tools:    a.tools.ToolDefs(),
		}

		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("provider error: %w", err)
		}

		a.session.AddMessage(cobot.Message{
			Role:      cobot.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			return resp, nil
		}

		results := a.tools.ExecuteParallel(ctx, resp.ToolCalls)
		for _, tr := range results {
			a.session.AddMessage(cobot.Message{
				Role:       cobot.RoleTool,
				ToolResult: tr,
			})
		}
	}

	return nil, cobot.ErrMaxTurnsExceeded
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan cobot.Event, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}

	ch := make(chan cobot.Event, 64)

	go func() {
		defer close(ch)
		a.session.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

		for turn := 0; turn < a.config.MaxTurns; turn++ {
			msgs := a.session.Messages()
			req := &cobot.ProviderRequest{
				Model:    a.config.Model,
				Messages: msgs,
				Tools:    a.tools.ToolDefs(),
			}

			streamCh, err := a.provider.Stream(ctx, req)
			if err != nil {
				ch <- cobot.Event{Type: cobot.EventError, Error: err}
				return
			}

			var content string
			var toolCalls []cobot.ToolCall
			for chunk := range streamCh {
				if chunk.Content != "" {
					content += chunk.Content
					ch <- cobot.Event{Type: cobot.EventText, Content: chunk.Content}
				}
				if chunk.ToolCall != nil {
					toolCalls = append(toolCalls, *chunk.ToolCall)
				}
				if chunk.Done && len(toolCalls) == 0 {
					a.session.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content})
					ch <- cobot.Event{Type: cobot.EventDone, Done: true}
					return
				}
			}

			if len(toolCalls) > 0 {
				a.session.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content, ToolCalls: toolCalls})
				results := a.tools.ExecuteParallel(ctx, toolCalls)
				for _, tr := range results {
					ch <- cobot.Event{Type: cobot.EventToolResult, Content: tr.Output}
					a.session.AddMessage(cobot.Message{Role: cobot.RoleTool, ToolResult: tr})
				}
			}
		}

		ch <- cobot.Event{Type: cobot.EventError, Error: cobot.ErrMaxTurnsExceeded}
	}()

	return ch, nil
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./internal/agent/ -v`
Expected: all 3 tests PASS

- [ ] **Step 8: Commit**

```bash
git add internal/agent/
git commit -m "feat: add agent loop with think-act-observe cycle"
```

---

### Task 10: Wire SDK Entry Point to Agent Loop

**Files:**
- Modify: `pkg/cobot.go`
- Test: `pkg/cobot_test.go`

- [ ] **Step 1: Write failing test for SDK usage**

```go
package cobot

import (
	"context"
	"encoding/json"
	"testing"
)

type testProvider struct {
	response string
}

func (t *testProvider) Name() string { return "test" }
func (t *testProvider) Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error) {
	return &ProviderResponse{Content: t.response, StopReason: StopEndTurn}, nil
}
func (t *testProvider) Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error) {
	ch := make(chan ProviderChunk, 1)
	ch <- ProviderChunk{Content: t.response, Done: true}
	close(ch)
	return ch, nil
}

func TestNewAgent(t *testing.T) {
	cfg := DefaultConfig()
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if agent == nil {
		t.Error("expected non-nil agent")
	}
}

func TestAgentPrompt(t *testing.T) {
	cfg := DefaultConfig()
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	agent.SetProvider(&testProvider{response: "Hello!"})

	resp, err := agent.Prompt(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("expected Hello!, got %s", resp.Content)
	}
}
```

- [ ] **Step 2: Update pkg/cobot.go to delegate to internal agent**

```go
package cobot

import (
	"context"

	"github.com/cobot-agent/cobot/internal/agent"
)

type Agent struct {
	inner    *agent.Agent
	config   *Config
	provider Provider
}

func New(config *Config) (*Agent, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &Agent{
		inner:  agent.New(config),
		config: config,
	}, nil
}

func (a *Agent) SetProvider(p Provider) {
	a.provider = p
	a.inner.SetProvider(p)
}

func (a *Agent) Prompt(ctx context.Context, message string) (*ProviderResponse, error) {
	return a.inner.Prompt(ctx, message)
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan Event, error) {
	return a.inner.Stream(ctx, message)
}

func (a *Agent) RegisterTool(tool Tool) error {
	a.inner.ToolRegistry().Register(tool)
	return nil
}

func (a *Agent) Close() error {
	return a.inner.Close()
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/ -v`
Expected: both tests PASS

- [ ] **Step 4: Verify full build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add pkg/
git commit -m "feat: wire SDK entry point to internal agent loop"
```

---

### Task 11: Cobra CLI with `cobot chat` Command

**Files:**
- Rewrite: `cmd/cobot/main.go`
- Create: `cmd/cobot/root.go`
- Create: `cmd/cobot/chat.go`
- Create: `cmd/cobot/model.go`
- Create: `cmd/cobot/workspace.go`

- [ ] **Step 1: Create cmd/cobot/root.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/internal/config"
)

var (
	cfgPath     string
	workspacePath string
	modelName   string
)

var rootCmd = &cobra.Command{
	Use:   "cobot",
	Short: "A personal AI agent system",
	Long:  "Cobot is a Go-based personal agent system with memory, tools, and protocols.",
	Version: "0.1.0",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", "", "workspace directory")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "LLM model (e.g. openai:gpt-4o)")
}

func loadConfig() (*cobot.Config, error) {
	cfg := cobot.DefaultConfig()
	if cfgPath != "" {
		if err := config.LoadFromFile(cfg, cfgPath); err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	}
	if modelName != "" {
		cfg.Model = modelName
	}
	if workspacePath != "" {
		cfg.Workspace = workspacePath
	}
	return cfg, nil
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
```

- [ ] **Step 2: Create cmd/cobot/chat.go**

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/internal/llm/openai"
)

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a message to the agent",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		agent, err := cobot.New(cfg)
		if err != nil {
			return err
		}
		defer agent.Close()

		apiKey := cfg.APIKeys["openai"]
		if apiKey == "" {
			return fmt.Errorf("openai API key not configured (set api_keys.openai in config or OPENAI_API_KEY env)")
		}

		provider := openai.NewProvider(apiKey, "")
		agent.SetProvider(provider)

		ch, err := agent.Stream(context.Background(), args[0])
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
```

- [ ] **Step 3: Create cmd/cobot/model.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "List or set the active model",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available models:")
		fmt.Println("  openai:gpt-4o")
		fmt.Println("  openai:gpt-4o-mini")
		fmt.Println("  openrouter:auto")
		return nil
	},
}

func init() {
	modelCmd.AddCommand(modelListCmd)
	rootCmd.AddCommand(chatCmd, modelCmd)
}
```

- [ ] **Step 4: Create cmd/cobot/workspace.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cobot-agent/cobot/internal/workspace"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

var workspaceInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .cobot workspace in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if workspacePath != "" {
			dir = workspacePath
		}
		ws, err := workspace.Init(dir)
		if err != nil {
			return err
		}
		fmt.Printf("Initialized cobot workspace at %s\n", ws.Root)
		return nil
	},
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Workspace list not yet implemented (Phase 4)")
		return nil
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceInitCmd, workspaceListCmd)
	rootCmd.AddCommand(workspaceCmd)
}
```

- [ ] **Step 5: Delete cmd/cobot/main.go** (root.go has main())

Since root.go contains `main()`, the old main.go is no longer needed. Delete it:

```bash
rm cmd/cobot/main.go
```

- [ ] **Step 6: Build and verify**

Run: `go build ./cmd/cobot/`
Expected: builds without errors

- [ ] **Step 7: Test CLI help**

Run: `./cobot --help`
Expected: shows usage with chat, model, workspace commands

- [ ] **Step 8: Test workspace init**

Run: `./cobot workspace init`
Expected: creates `.cobot/` directory

- [ ] **Step 9: Commit**

```bash
git add cmd/
git commit -m "feat: add cobra CLI with chat, model, and workspace commands"
```

---

### Task 12: End-to-End Integration Test

**Files:**
- Create: `internal/agent/e2e_test.go`

- [ ] **Step 1: Write end-to-end test exercising the full stack**

```go
package agent

import (
	"context"
	"encoding/json"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/internal/tools/builtin"
)

func TestE2ESimpleConversation(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{Content: "I can help with that!", StopReason: cobot.StopEndTurn},
		},
	})

	resp, err := a.Prompt(context.Background(), "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "I can help with that!" {
		t.Errorf("unexpected: %s", resp.Content)
	}
	if len(a.Session().Messages()) != 2 {
		t.Errorf("expected 2 messages (user + assistant), got %d", len(a.Session().Messages()))
	}
}

func TestE2EToolCallFlow(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"})
	a.ToolRegistry().Register(builtin.NewShellExecTool())

	a.SetProvider(&multiCallProvider{
		responses: []*cobot.ProviderResponse{
			{
				ToolCalls: []cobot.ToolCall{{
					ID:       "call_1",
					Name:     "shell_exec",
					Arguments: json.RawMessage(`{"command":"echo hello"}`),
				}},
				StopReason: cobot.StopEndTurn,
			},
			{Content: "The shell command output: hello", StopReason: cobot.StopEndTurn},
		},
	})

	resp, err := a.Prompt(context.Background(), "run echo hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "The shell command output: hello" {
		t.Errorf("unexpected: %s", resp.Content)
	}
	msgs := a.Session().Messages()
	if len(msgs) != 4 {
		t.Errorf("expected 4 messages (user, assistant+toolcall, tool, assistant), got %d", len(msgs))
	}
}

type multiCallProvider struct {
	responses []*cobot.ProviderResponse
	calls     int
}

func (m *multiCallProvider) Name() string { return "multi-call-mock" }
func (m *multiCallProvider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	if m.calls >= len(m.responses) {
		return &cobot.ProviderResponse{Content: "done", StopReason: cobot.StopEndTurn}, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}
func (m *multiCallProvider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	ch := make(chan cobot.ProviderChunk, 1)
	resp, err := m.Complete(ctx, req)
	if err != nil {
		close(ch)
		return ch, err
	}
	ch <- cobot.ProviderChunk{Content: resp.Content, Done: true}
	close(ch)
	return ch, nil
}

func TestE2EStreaming(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{Content: "Streaming response", StopReason: cobot.StopEndTurn},
		},
	})

	ch, err := a.Stream(context.Background(), "stream test")
	if err != nil {
		t.Fatal(err)
	}

	var collected string
	for event := range ch {
		if event.Type == cobot.EventText {
			collected += event.Content
		}
	}
	if collected != "Streaming response" {
		t.Errorf("unexpected streaming output: %s", collected)
	}
}
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -v`
Expected: all tests PASS across all packages

- [ ] **Step 3: Commit**

```bash
git add internal/agent/e2e_test.go
git commit -m "test: add end-to-end integration tests for agent loop"
```

---

### Task 13: Final Verification and Cleanup

- [ ] **Step 1: Run full build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 2: Run all tests**

Run: `go test ./... -v -count=1`
Expected: all tests PASS

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: Test CLI binary end-to-end**

```bash
go run ./cmd/cobot/ --help
go run ./cmd/cobot/ version
go run ./cmd/cobot/ workspace init -w /tmp/test-workspace
ls /tmp/test-workspace/.cobot/
```

Expected: CLI shows help, version, and creates workspace

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: phase 1 foundation complete — config, provider, agent loop, tools, cli"
```
