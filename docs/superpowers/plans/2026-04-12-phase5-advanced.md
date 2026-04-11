# Phase 5: Advanced — Skills, Scheduler, Anthropic Provider, Memory Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the final layer — YAML-defined skill system with slash command triggers, cron-based scheduler for automated prompts, Anthropic LLM provider, and memory performance optimization (persistent Bleve index).

**Architecture:** Skills are YAML files loaded from `.cobot/skills/` with a step-by-step executor that runs prompts through the agent. Scheduler uses `robfig/cron` to execute prompts on cron schedules and store results to memory or file. Anthropic provider follows the same `cobot.Provider` interface as OpenAI. Memory optimization replaces per-operation Bleve open/close with a persistent index handle.

**Tech Stack:** `github.com/robfig/cron/v3` (scheduler), `gopkg.in/yaml.v3` (skill loading), Anthropic Messages API (provider), existing BadgerDB + Bleve stack

**Design Spec:** `docs/specs/2026-04-12-cobot-agent-system-design.md` Sections 11, 12, 15.7

---

## Phase 4 Outstanding Issues (Tracked, Not Blocking)

| # | Severity | Issue |
|---|----------|-------|
| P4-I4 | Important | Chat command sets memory on core but SDK Agent.memStore stays nil |
| P4-I5 | Important | truncate() counts bytes not runes |
| P4-I6 | Important | Hardcoded tool list in tools.go |
| P4-I7 | Important | config show doesn't mask ProviderConfig.Headers/MCPServerConfig |
| P3-I2 | Important | MCPManager.Close() swallows errors |
| P3-I3 | Important | buildSystemPrompt swallows WakeUp errors |
| P3-I4 | Important | ListTools uses full lock instead of RLock |
| P3-I5 | Important | CancelAll doesn't cancel subagent contexts |
| P3-I6 | Important | KG key collision with colon delimiter |

---

## File Structure

```
internal/
├── skills/
│   ├── skill.go           # Skill, Step types + Loader
│   ├── executor.go        # Executor: runs skill steps through agent
│   ├── executor_test.go   # Executor tests
│   └── loader_test.go     # Loader tests with YAML fixtures
├── scheduler/
│   ├── scheduler.go       # Scheduler struct, Start/Stop/AddTask/RemoveTask/ListTasks
│   ├── types.go           # Task, TaskOutput types
│   └── scheduler_test.go  # Scheduler tests
├── llm/
│   └── anthropic/
│       ├── types.go       # Anthropic API request/response types
│       ├── provider.go    # Provider implementing cobot.Provider
│       └── provider_test.go
└── memory/
    └── search.go          # MODIFY: persistent Bleve index

cmd/cobot/
├── scheduler_cmd.go       # NEW: scheduler list/add commands
└── root.go                # MODIFY: register scheduler

docs/
└── skills/
    └── example-review.yaml  # Example skill definition
```

---

### Task 1: Skill System — Types + Loader

**Files:**
- Create: `internal/skills/skill.go`
- Create: `internal/skills/loader_test.go`

**Goal:** Define Skill/Step types and YAML loader.

- [ ] **Step 1: Create `internal/skills/skill.go`**

```go
package skills

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

type Skill struct {
    Name        string         `yaml:"name"`
    Description string         `yaml:"description"`
    Trigger     string         `yaml:"trigger"`
    Steps       []Step         `yaml:"steps"`
}

type Step struct {
    Prompt string         `yaml:"prompt"`
    Tool   string         `yaml:"tool,omitempty"`
    Args   map[string]any `yaml:"args,omitempty"`
    Output string         `yaml:"output,omitempty"`
}

func LoadFromYAML(path string) (*Skill, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read skill %s: %w", path, err)
    }
    var skill Skill
    if err := yaml.Unmarshal(data, &skill); err != nil {
        return nil, fmt.Errorf("parse skill %s: %w", path, err)
    }
    if skill.Name == "" {
        skill.Name = filepath.Base(path)
    }
    return &skill, nil
}

func LoadFromDir(dir string) ([]*Skill, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    var skills []*Skill
    for _, entry := range entries {
        if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
            continue
        }
        skill, err := LoadFromYAML(filepath.Join(dir, entry.Name()))
        if err != nil {
            return nil, err
        }
        skills = append(skills, skill)
    }
    return skills, nil
}
```

- [ ] **Step 2: Create `internal/skills/loader_test.go`**

```go
package skills

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadFromYAML(t *testing.T) {
    dir := t.TempDir()
    yamlData := `name: test-skill
description: A test skill
trigger: /test
steps:
  - prompt: "Hello world"
    output: greeting
`
    path := filepath.Join(dir, "test.yaml")
    os.WriteFile(path, []byte(yamlData), 0644)

    skill, err := LoadFromYAML(path)
    if err != nil {
        t.Fatal(err)
    }
    if skill.Name != "test-skill" {
        t.Errorf("expected test-skill, got %s", skill.Name)
    }
    if skill.Trigger != "/test" {
        t.Errorf("expected /test, got %s", skill.Trigger)
    }
    if len(skill.Steps) != 1 {
        t.Fatalf("expected 1 step, got %d", len(skill.Steps))
    }
    if skill.Steps[0].Prompt != "Hello world" {
        t.Errorf("unexpected prompt: %s", skill.Steps[0].Prompt)
    }
    if skill.Steps[0].Output != "greeting" {
        t.Errorf("unexpected output: %s", skill.Steps[0].Output)
    }
}

func TestLoadFromYAMLNoName(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "myskill.yaml")
    os.WriteFile(path, []byte("trigger: /my\n"), 0644)

    skill, err := LoadFromYAML(path)
    if err != nil {
        t.Fatal(err)
    }
    if skill.Name != "myskill.yaml" {
        t.Errorf("expected myskill.yaml, got %s", skill.Name)
    }
}

func TestLoadFromDir(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("name: a\ntrigger: /a\n"), 0644)
    os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("name: b\ntrigger: /b\n"), 0644)
    os.WriteFile(filepath.Join(dir, "c.txt"), []byte("not a skill"), 0644)

    skills, err := LoadFromDir(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(skills) != 2 {
        t.Fatalf("expected 2 skills, got %d", len(skills))
    }
}

func TestLoadFromDirMissing(t *testing.T) {
    skills, err := LoadFromDir("/nonexistent")
    if err != nil {
        t.Fatal("expected nil error for missing dir")
    }
    if skills != nil {
        t.Error("expected nil skills")
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/skills/... -v`

- [ ] **Step 4: Commit**

```bash
git add internal/skills/
git commit -m "feat: add skill system types and YAML loader"
```

---

### Task 2: Skill System — Executor

**Files:**
- Create: `internal/skills/executor.go`
- Create: `internal/skills/executor_test.go`

**Goal:** Executor runs skill steps through the agent sequentially.

- [ ] **Step 1: Create `internal/skills/executor.go`**

```go
package skills

import (
    "context"
    "fmt"
    "strings"

    "github.com/cobot-agent/cobot/internal/agent"
)

type Executor struct {
    agent *agent.Agent
}

func NewExecutor(a *agent.Agent) *Executor {
    return &Executor{agent: a}
}

func (e *Executor) Execute(ctx context.Context, skill *Skill, input string) (string, error) {
    var results []string
    for i, step := range skill.Steps {
        prompt := step.Prompt
        if i == 0 && input != "" {
            prompt = prompt + "\n\n" + input
        }

        resp, err := e.agent.Prompt(ctx, prompt)
        if err != nil {
            return strings.Join(results, "\n---\n"), fmt.Errorf("step %d (%s): %w", i, step.Output, err)
        }

        if step.Output != "" {
            results = append(results, fmt.Sprintf("[%s]\n%s", step.Output, resp.Content))
        }
    }

    if len(results) == 0 {
        return "Skill completed with no output steps", nil
    }
    return strings.Join(results, "\n---\n"), nil
}
```

- [ ] **Step 2: Create `internal/skills/executor_test.go`**

```go
package skills

import (
    "context"
    "testing"

    "github.com/cobot-agent/cobot/internal/agent"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type mockProvider struct {
    responses []*cobot.ProviderResponse
    index     int
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
    if m.index >= len(m.responses) {
        return &cobot.ProviderResponse{Content: "done", StopReason: cobot.StopEndTurn}, nil
    }
    resp := m.responses[m.index]
    m.index++
    return resp, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
    return nil, nil
}

func TestExecutorBasicSkill(t *testing.T) {
    a := agent.New(&cobot.Config{Model: "mock", MaxTurns: 5})
    a.SetProvider(&mockProvider{
        responses: []*cobot.ProviderResponse{
            {Content: "Step 1 result", StopReason: cobot.StopEndTurn},
            {Content: "Step 2 result", StopReason: cobot.StopEndTurn},
        },
    })

    skill := &Skill{
        Name:    "test",
        Trigger: "/test",
        Steps: []Step{
            {Prompt: "Do step 1", Output: "step1"},
            {Prompt: "Do step 2", Output: "step2"},
        },
    }

    exec := NewExecutor(a)
    result, err := exec.Execute(context.Background(), skill, "extra input")
    if err != nil {
        t.Fatal(err)
    }
    if result == "" {
        t.Error("expected non-empty result")
    }
    if !contains(result, "step1") {
        t.Error("expected step1 in result")
    }
    if !contains(result, "step2") {
        t.Error("expected step2 in result")
    }
}

func TestExecutorNoOutput(t *testing.T) {
    a := agent.New(&cobot.Config{Model: "mock", MaxTurns: 5})
    a.SetProvider(&mockProvider{
        responses: []*cobot.ProviderResponse{
            {Content: "done", StopReason: cobot.StopEndTurn},
        },
    })

    skill := &Skill{
        Name:    "nooutput",
        Trigger: "/nooutput",
        Steps: []Step{
            {Prompt: "Do something"},
        },
    }

    exec := NewExecutor(a)
    result, err := exec.Execute(context.Background(), skill, "")
    if err != nil {
        t.Fatal(err)
    }
    if result != "Skill completed with no output steps" {
        t.Errorf("unexpected: %s", result)
    }
}

func contains(s, sub string) bool {
    return len(s) >= len(sub) && (s == sub || len(sub) == 0 || strings.Contains(s, sub))
}
```

Add `"strings"` to imports.

Wait, `contains` uses `strings.Contains`. Import it:

```go
import (
    "strings"
    "testing"
    ...
)
```

Actually the `contains` helper can just use `strings.Contains` directly. Let me simplify:

```go
func TestExecutorBasicSkill(t *testing.T) {
    ...
    if !strings.Contains(result, "step1") {
        t.Error("expected step1 in result")
    }
}
```

Remove the `contains` helper, use `strings.Contains` directly. Add `"strings"` to test imports.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/skills/... -v`

- [ ] **Step 4: Commit**

```bash
git add internal/skills/
git commit -m "feat: add skill executor that runs steps through agent"
```

---

### Task 3: Scheduler

**Files:**
- Create: `internal/scheduler/types.go`
- Create: `internal/scheduler/scheduler.go`
- Create: `internal/scheduler/scheduler_test.go`
- Create: `cmd/cobot/scheduler_cmd.go`

**Goal:** Cron-based scheduler for automated prompts.

- [ ] **Step 1: Add cron dependency**

```bash
go get github.com/robfig/cron/v3@latest
```

- [ ] **Step 2: Create `internal/scheduler/types.go`**

```go
package scheduler

type Task struct {
    Name      string `yaml:"name" json:"name"`
    Schedule  string `yaml:"schedule" json:"schedule"`
    Prompt    string `yaml:"prompt" json:"prompt"`
    Output    string `yaml:"output" json:"output"`
    OutputPath string `yaml:"output_path" json:"output_path,omitempty"`
}
```

- [ ] **Step 3: Create `internal/scheduler/scheduler.go`**

```go
package scheduler

import (
    "context"
    "fmt"
    "log/slog"
    "sync"

    "github.com/robfig/cron/v3"

    "github.com/cobot-agent/cobot/internal/agent"
)

type Scheduler struct {
    agent *agent.Agent
    cron  *cron.Cron
    mu    sync.RWMutex
    ids   map[string]cron.EntryID
    tasks map[string]*Task
}

func New(a *agent.Agent) *Scheduler {
    return &Scheduler{
        agent: a,
        cron:  cron.New(cron.WithSeconds()),
        ids:   make(map[string]cron.EntryID),
        tasks: make(map[string]*Task),
    }
}

func (s *Scheduler) Start() error {
    s.cron.Start()
    return nil
}

func (s *Scheduler) Stop() context.Context {
    return s.cron.Stop()
}

func (s *Scheduler) AddTask(task *Task) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if _, exists := s.ids[task.Name]; exists {
        return fmt.Errorf("task %q already exists", task.Name)
    }

    id, err := s.cron.AddFunc(task.Schedule, func() {
        slog.Info("scheduler: executing task", "name", task.Name)
        resp, err := s.agent.Prompt(context.Background(), task.Prompt)
        if err != nil {
            slog.Error("scheduler: task failed", "name", task.Name, "error", err)
            return
        }
        if task.Output == "memory" && s.agent.MemoryStore() != nil {
            s.agent.MemoryStore().Store(context.Background(), resp.Content, "scheduler", task.Name)
        }
        slog.Info("scheduler: task completed", "name", task.Name)
    })
    if err != nil {
        return fmt.Errorf("parse schedule %q: %w", task.Schedule, err)
    }

    s.ids[task.Name] = id
    s.tasks[task.Name] = task
    return nil
}

func (s *Scheduler) RemoveTask(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    id, ok := s.ids[name]
    if !ok {
        return fmt.Errorf("task %q not found", name)
    }
    s.cron.Remove(id)
    delete(s.ids, name)
    delete(s.tasks, name)
    return nil
}

func (s *Scheduler) ListTasks() []*Task {
    s.mu.RLock()
    defer s.mu.RUnlock()
    tasks := make([]*Task, 0, len(s.tasks))
    for _, t := range s.tasks {
        tasks = append(tasks, t)
    }
    return tasks
}
```

- [ ] **Step 4: Create `internal/scheduler/scheduler_test.go`**

```go
package scheduler

import (
    "testing"

    "github.com/cobot-agent/cobot/internal/agent"
    cobot "github.com/cobot-agent/cobot/pkg"
)

func TestNewScheduler(t *testing.T) {
    a := agent.New(cobot.DefaultConfig())
    s := New(a)
    if s == nil {
        t.Fatal("expected scheduler")
    }
}

func TestAddTask(t *testing.T) {
    a := agent.New(cobot.DefaultConfig())
    s := New(a)

    err := s.AddTask(&Task{
        Name:     "test",
        Schedule: "0 0 * * * *",
        Prompt:   "test prompt",
    })
    if err != nil {
        t.Fatal(err)
    }

    tasks := s.ListTasks()
    if len(tasks) != 1 {
        t.Fatalf("expected 1 task, got %d", len(tasks))
    }
    if tasks[0].Name != "test" {
        t.Errorf("expected test, got %s", tasks[0].Name)
    }
}

func TestAddTaskDuplicate(t *testing.T) {
    a := agent.New(cobot.DefaultConfig())
    s := New(a)

    s.AddTask(&Task{Name: "dup", Schedule: "0 0 * * * *", Prompt: "x"})
    err := s.AddTask(&Task{Name: "dup", Schedule: "0 0 * * * *", Prompt: "y"})
    if err == nil {
        t.Error("expected error for duplicate task")
    }
}

func TestRemoveTask(t *testing.T) {
    a := agent.New(cobot.DefaultConfig())
    s := New(a)

    s.AddTask(&Task{Name: "remove-me", Schedule: "0 0 * * * *", Prompt: "x"})
    err := s.RemoveTask("remove-me")
    if err != nil {
        t.Fatal(err)
    }
    if len(s.ListTasks()) != 0 {
        t.Error("expected 0 tasks after remove")
    }
}

func TestRemoveTaskNotFound(t *testing.T) {
    a := agent.New(cobot.DefaultConfig())
    s := New(a)

    err := s.RemoveTask("nonexistent")
    if err == nil {
        t.Error("expected error for nonexistent task")
    }
}

func TestAddTaskBadSchedule(t *testing.T) {
    a := agent.New(cobot.DefaultConfig())
    s := New(a)

    err := s.AddTask(&Task{Name: "bad", Schedule: "not-a-cron", Prompt: "x"})
    if err == nil {
        t.Error("expected error for bad cron spec")
    }
}
```

- [ ] **Step 5: Create `cmd/cobot/scheduler_cmd.go`**

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/spf13/cobra"
)

var schedulerCmd = &cobra.Command{
    Use:   "scheduler",
    Short: "Manage scheduled tasks",
}

var schedulerListCmd = &cobra.Command{
    Use:   "list",
    Short: "List scheduled tasks",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Fprintln(cmd.OutOrStdout(), "No scheduler running. Start via config or TUI.")
        return nil
    },
}

func init() {
    schedulerCmd.AddCommand(schedulerListCmd)
    rootCmd.AddCommand(schedulerCmd)
}
```

Note: Full scheduler integration with config loading and persistent execution is deferred — the CLI is a skeleton for now. The `internal/scheduler/` package is the real implementation.

- [ ] **Step 6: Run tests**

Run: `go mod tidy && go test ./internal/scheduler/... -v`

- [ ] **Step 7: Commit**

```bash
git add internal/scheduler/ cmd/cobot/scheduler_cmd.go go.mod go.sum
git commit -m "feat: add cron scheduler with add/remove/list tasks"
```

---

### Task 4: Anthropic Provider

**Files:**
- Create: `internal/llm/anthropic/types.go`
- Create: `internal/llm/anthropic/provider.go`
- Create: `internal/llm/anthropic/provider_test.go`

**Goal:** Anthropic Messages API provider implementing `cobot.Provider`.

- [ ] **Step 1: Create `internal/llm/anthropic/types.go`**

```go
package anthropic

import "encoding/json"

type messagesRequest struct {
    Model     string          `json:"model"`
    MaxTokens int             `json:"max_tokens"`
    Messages  []message       `json:"messages"`
    System    string          `json:"system,omitempty"`
    Tools     []toolDef       `json:"tools,omitempty"`
    Stream    bool            `json:"stream"`
}

type message struct {
    Role    string          `json:"role"`
    Content json.RawMessage `json:"content"`
}

type textBlock struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type toolUseBlock struct {
    Type  string          `json:"type"`
    ID    string          `json:"id"`
    Name  string          `json:"name"`
    Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
    Type      string `json:"type"`
    ToolUseID string `json:"tool_use_id"`
    Content   string `json:"content"`
}

type toolDef struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"input_schema"`
}

type messagesResponse struct {
    ID       string        `json:"id"`
    Type     string        `json:"type"`
    Role     string        `json:"role"`
    Content  []contentBlock `json:"content"`
    Model    string        `json:"model"`
    StopReason string      `json:"stop_reason"`
    Usage    usage         `json:"usage"`
}

type contentBlock struct {
    Type string          `json:"type"`
    Text string          `json:"text,omitempty"`
    ID   string          `json:"id,omitempty"`
    Name string          `json:"name,omitempty"`
    Input json.RawMessage `json:"input,omitempty"`
}

type usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

type streamEvent struct {
    Type         string        `json:"type"`
    Index        int           `json:"index,omitempty"`
    Delta        *streamDelta  `json:"delta,omitempty"`
    ContentBlock *contentBlock `json:"content_block,omitempty"`
    Message      *messagesResponse `json:"message,omitempty"`
}

type streamDelta struct {
    Type        string          `json:"type,omitempty"`
    Text        string          `json:"text,omitempty"`
    PartialJSON string          `json:"partial_json,omitempty"`
    StopReason  string          `json:"stop_reason,omitempty"`
}
```

- [ ] **Step 2: Create `internal/llm/anthropic/provider.go`**

```go
package anthropic

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"

    cobot "github.com/cobot-agent/cobot/pkg"
)

type Provider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

func NewProvider(apiKey, baseURL string) *Provider {
    baseURL = strings.TrimRight(baseURL, "/")
    if baseURL == "" {
        baseURL = "https://api.anthropic.com"
    }
    return &Provider{
        apiKey:  apiKey,
        baseURL: baseURL,
        client:  &http.Client{},
    }
}

func (p *Provider) Name() string { return "anthropic" }

func (p *Provider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
    body := p.buildRequest(req, false)
    respBody, err := p.doRequest(ctx, body)
    if err != nil {
        return nil, err
    }
    defer respBody.Close()

    var resp messagesResponse
    if err := json.NewDecoder(respBody).Decode(&resp); err != nil {
        return nil, fmt.Errorf("anthropic: decode response: %w", err)
    }

    return p.toProviderResponse(&resp), nil
}

func (p *Provider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
    body := p.buildRequest(req, true)
    respBody, err := p.doRequest(ctx, body)
    if err != nil {
        return nil, err
    }

    ch := make(chan cobot.ProviderChunk, 64)
    go func() {
        defer close(ch)
        defer respBody.Close()
        p.readStream(respBody, ch)
    }()
    return ch, nil
}

func (p *Provider) buildRequest(req *cobot.ProviderRequest, stream bool) messagesRequest {
    var system string
    var msgs []message
    for _, m := range req.Messages {
        if m.Role == cobot.RoleSystem {
            system = m.Content
            continue
        }
        content, _ := json.Marshal(textBlock{Type: "text", Text: m.Content})
        msgs = append(msgs, message{Role: string(m.Role), Content: content})
    }

    var tools []toolDef
    for _, t := range req.Tools {
        tools = append(tools, toolDef{
            Name:        t.Name,
            Description: t.Description,
            InputSchema: t.Parameters,
        })
    }

    maxTokens := req.MaxTokens
    if maxTokens == 0 {
        maxTokens = 4096
    }

    return messagesRequest{
        Model:     strings.TrimPrefix(req.Model, "anthropic:"),
        MaxTokens: maxTokens,
        Messages:  msgs,
        System:    system,
        Tools:     tools,
        Stream:    stream,
    }
}

func (p *Provider) doRequest(ctx context.Context, body messagesRequest) (io.ReadCloser, error) {
    jsonBody, err := json.Marshal(body)
    if err != nil {
        return nil, fmt.Errorf("anthropic: marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
    if err != nil {
        return nil, fmt.Errorf("anthropic: create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", p.apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("anthropic: request failed: %w", err)
    }
    if resp.StatusCode != http.StatusOK {
        defer resp.Body.Close()
        data, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("anthropic: API error %d: %s", resp.StatusCode, string(data))
    }
    return resp.Body, nil
}

func (p *Provider) toProviderResponse(resp *messagesResponse) *cobot.ProviderResponse {
    var content string
    var toolCalls []cobot.ToolCall
    for _, block := range resp.Content {
        if block.Type == "text" {
            content += block.Text
        }
    }
    stopReason := cobot.StopEndTurn
    if resp.StopReason == "max_tokens" {
        stopReason = cobot.StopMaxTokens
    } else if resp.StopReason == "tool_use" {
        stopReason = cobot.StopEndTurn
    }
    return &cobot.ProviderResponse{
        Content:    content,
        ToolCalls:  toolCalls,
        StopReason: stopReason,
        Usage: cobot.Usage{
            PromptTokens:     resp.Usage.InputTokens,
            CompletionTokens: resp.Usage.OutputTokens,
            TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
        },
    }
}

func (p *Provider) readStream(body io.ReadCloser, ch chan<- cobot.ProviderChunk) {
    scanner := bufio.NewScanner(body)
    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "event: ") {
            if strings.HasPrefix(line, "data: ") {
                var evt streamEvent
                json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt)
                if evt.Delta != nil && evt.Delta.Text != "" {
                    ch <- cobot.ProviderChunk{Content: evt.Delta.Text}
                }
                if evt.Delta != nil && evt.Delta.StopReason != "" {
                    ch <- cobot.ProviderChunk{Done: true}
                    return
                }
            }
            continue
        }
    }
    ch <- cobot.ProviderChunk{Done: true}
}
```

- [ ] **Step 3: Create `internal/llm/anthropic/provider_test.go`**

```go
package anthropic

import (
    "testing"
)

func TestNewProvider(t *testing.T) {
    p := NewProvider("sk-test", "")
    if p.Name() != "anthropic" {
        t.Errorf("expected anthropic, got %s", p.Name())
    }
}

func TestNewProviderCustomBaseURL(t *testing.T) {
    p := NewProvider("key", "https://custom.api.com/")
    if p.baseURL != "https://custom.api.com" {
        t.Errorf("expected trimmed URL, got %s", p.baseURL)
    }
}

func TestBuildRequest(t *testing.T) {
    p := NewProvider("key", "")
    req := &cobot.ProviderRequest{
        Model:    "claude-3-sonnet",
        Messages: []cobot.Message{
            {Role: cobot.RoleSystem, Content: "You are helpful."},
            {Role: cobot.RoleUser, Content: "Hello"},
        },
    }
    body := p.buildRequest(req, false)
    if body.Model != "claude-3-sonnet" {
        t.Errorf("expected claude-3-sonnet, got %s", body.Model)
    }
    if body.System != "You are helpful." {
        t.Errorf("expected system prompt, got %s", body.System)
    }
    if len(body.Messages) != 1 {
        t.Fatalf("expected 1 message, got %d", len(body.Messages))
    }
    if body.Stream {
        t.Error("expected stream false")
    }
    if body.MaxTokens != 4096 {
        t.Errorf("expected 4096 max tokens, got %d", body.MaxTokens)
    }
}

func TestBuildRequestModelPrefix(t *testing.T) {
    p := NewProvider("key", "")
    req := &cobot.ProviderRequest{Model: "anthropic:claude-3-opus"}
    body := p.buildRequest(req, false)
    if body.Model != "claude-3-opus" {
        t.Errorf("expected prefix stripped, got %s", body.Model)
    }
}
```

Add `cobot "github.com/cobot-agent/cobot/pkg"` import.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/llm/anthropic/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/llm/anthropic/
git commit -m "feat: add Anthropic Messages API provider"
```

---

### Task 5: Memory Performance — Persistent Bleve Index

**Files:**
- Modify: `internal/memory/badger.go`
- Modify: `internal/memory/store.go`
- Modify: `internal/memory/search.go`
- Modify: `internal/memory/store_test.go`

**Goal:** Replace per-operation Bleve open/close with a persistent index handle stored on the Store struct.

- [ ] **Step 1: Read current files**

Read `internal/memory/search.go`, `internal/memory/store.go`, `internal/memory/badger.go`.

- [ ] **Step 2: Add bleveIndex field to Store**

In `store.go`, add `bleveIdx bleve.Index` field. Modify `OpenStore` to open the Bleve index once. Modify `Close` to close it.

Current Store:
```go
type Store struct {
    db       *badger.DB
    bleveDir string
}
```

Change to:
```go
type Store struct {
    db       *badger.DB
    bleveDir string
    bleveIdx bleve.Index
}
```

Modify `OpenStore` to initialize the index:
```go
func OpenStore(memoryDir string) (*Store, error) {
    dbPath := filepath.Join(memoryDir, "badger")
    db, err := openBadger(dbPath)
    if err != nil {
        return nil, err
    }
    bleveDir := filepath.Join(memoryDir, "bleve")
    os.MkdirAll(bleveDir, 0755)
    idx, err := openIndex(bleveDir)
    if err != nil {
        db.Close()
        return nil, err
    }
    return &Store{db: db, bleveDir: bleveDir, bleveIdx: idx}, nil
}
```

Modify `Close`:
```go
func (s *Store) Close() error {
    if s.bleveIdx != nil {
        s.bleveIdx.Close()
    }
    if s.db != nil {
        return s.db.Close()
    }
    return nil
}
```

- [ ] **Step 3: Modify search.go**

Remove `openIndex()` calls from `indexDrawer` and `searchDrawers`. Use `s.bleveIdx` directly.

Change `indexDrawer` from:
```go
func (s *Store) indexDrawer(doc *drawerDoc) error {
    idx, err := openIndex(s.bleveDir)
    ...
    idx.Index(doc.ID, doc)
    idx.Close()
}
```

To:
```go
func (s *Store) indexDrawer(doc *drawerDoc) error {
    return s.bleveIdx.Index(doc.ID, doc)
}
```

Change `searchDrawers` similarly:
```go
func (s *Store) searchDrawers(ctx context.Context, query *cobot.SearchQuery) ([]*cobot.SearchResult, error) {
    req := bleve.NewSearchRequest(...)
    ...
    result, err := s.bleveIdx.Search(req)
    ...
}
```

Keep `openIndex` function for use in `OpenStore` only.

- [ ] **Step 4: Run all memory tests**

Run: `go test ./internal/memory/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/memory/
git commit -m "perf: persistent Bleve index instead of per-operation open/close"
```

---

### Task 6: Final Verification

- [ ] **Step 1:** `go mod tidy`
- [ ] **Step 2:** `go build ./...`
- [ ] **Step 3:** `go vet ./...`
- [ ] **Step 4:** `go test ./... -count=1`
- [ ] **Step 5:** Verify all packages pass
- [ ] **Step 6:** Fix any issues
- [ ] **Step 7:** Final commit
