# Streaming Tool Calls & Real-time TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix OpenAI streaming tool call delta assembly (C1) and enable real-time incremental TUI rendering (I1).

**Architecture:** Two independent subsystems. (1) The OpenAI provider's `readStream` accumulates tool call deltas by `Index`, merging ID+name from first chunk with argument fragments from subsequent chunks, and emits a single assembled `ToolCall` when `finish_reason=tool_calls`. The agent loop's `Stream` emits `EventToolCall` events for each assembled tool call. (2) The TUI switches from a single blocking `tea.Cmd` to a per-chunk streaming pattern using a goroutine that sends each `Event` as a separate `streamMsg`, enabling real-time rendering.

**Tech Stack:** Go 1.26, Bubbletea (charmbracelet/bubbletea), lipgloss, existing cobot pkg types.

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/llm/openai/provider.go` | `readStream` — tool call delta assembly |
| Create | `internal/llm/openai/stream_test.go` | Unit tests for tool call delta assembly |
| Modify | `internal/agent/loop.go` | `Stream` — emit `EventToolCall`, fix assembly |
| Modify | `internal/agent/loop_test.go` | Add streaming tool call tests |
| Modify | `cmd/cobot/tui.go` | Real-time streaming, tool event display |

---

### Task 1: OpenAI Provider — Tool Call Delta Assembly

**Files:**
- Modify: `internal/llm/openai/provider.go:113-154`
- Create: `internal/llm/openai/stream_test.go`

**Background:** OpenAI sends tool calls across multiple SSE chunks. The first chunk for each tool call contains `id` + `function.name` + `index`. Subsequent chunks contain only `function.arguments` fragments with the same `index`. Current code treats each delta as a standalone `ToolCall`, producing garbage partial objects.

- [ ] **Step 1: Write the failing test for single tool call assembly**

Create `internal/llm/openai/stream_test.go`:

```go
package openai

import (
	"strings"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestReadStreamSingleToolCall(t *testing.T) {
	sseData := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"id":"chatpmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"ci"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"SF\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"data: [DONE]",
	}, "\n") + "\n"

	body := io.NopCloser(strings.NewReader(sseData))
	ch := make(chan cobot.ProviderChunk, 64)

	p := &Provider{}
	go func() {
		p.readStream(body, ch)
	}()

	var chunks []cobot.ProviderChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	doneIdx := -1
	var toolCallChunks []cobot.ProviderChunk
	for i, c := range chunks {
		if c.Done {
			doneIdx = i
		}
		if c.ToolCall != nil {
			toolCallChunks = append(toolCallChunks, c)
		}
	}

	if doneIdx < 0 {
		t.Fatal("expected a Done chunk")
	}

	if len(toolCallChunks) != 1 {
		t.Fatalf("expected exactly 1 assembled tool call chunk, got %d", len(toolCallChunks))
	}

	tc := toolCallChunks[0].ToolCall
	if tc.ID != "call_abc" {
		t.Errorf("expected ID call_abc, got %s", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("expected Name get_weather, got %s", tc.Name)
	}
	if string(tc.Arguments) != `{"city":"SF"}` {
		t.Errorf("expected assembled arguments {\"city\":\"SF\"}, got %s", string(tc.Arguments))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/openai/ -run TestReadStreamSingleToolCall -v`
Expected: FAIL — current code produces multiple partial tool call chunks instead of one assembled one.

- [ ] **Step 3: Implement tool call delta assembly in `readStream`**

Replace `internal/llm/openai/provider.go:113-154` with:

```go
func (p *Provider) readStream(body io.ReadCloser, ch chan<- cobot.ProviderChunk) {
	type pendingToolCall struct {
		ID   string
		Name string
		Args strings.Builder
	}
	pending := make(map[int]*pendingToolCall)

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- cobot.ProviderChunk{Done: true}
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			for _, tc := range choice.Delta.ToolCalls {
				p, exists := pending[tc.Index]
				if !exists {
					p = &pendingToolCall{}
					pending[tc.Index] = p
				}
				if tc.ID != "" {
					p.ID = tc.ID
				}
				if tc.Function != nil {
					if tc.Function.Name != "" {
						p.Name = tc.Function.Name
					}
					p.Args.WriteString(tc.Function.Arguments)
				}
			}

			pc := cobot.ProviderChunk{
				Content: choice.Delta.Content,
			}

			if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
				for i := 0; i < len(pending); i++ {
					ptc, ok := pending[i]
					if !ok {
						continue
					}
					ch <- cobot.ProviderChunk{
						ToolCall: &cobot.ToolCall{
							ID:        ptc.ID,
							Name:      ptc.Name,
							Arguments: json.RawMessage(ptc.Args.String()),
						},
					}
				}
			}

			if choice.FinishReason != nil {
				pc.Done = true
			}

			ch <- pc
		}
	}
}
```

Note: This requires adding `"strings"` to imports (already present). The key change is:
- `pending` map tracks tool calls by `Index`
- Each delta's `ID`, `Name`, `Arguments` fragments are accumulated
- When `finish_reason=tool_calls`, all pending tool calls are emitted as assembled `ToolCall` chunks
- Only the final `Done` chunk is sent after tool call emission

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/llm/openai/ -run TestReadStreamSingleToolCall -v`
Expected: PASS

- [ ] **Step 5: Write test for multiple parallel tool calls**

Add to `internal/llm/openai/stream_test.go`:

```go
func TestReadStreamMultipleToolCalls(t *testing.T) {
	sseData := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}},{"index":1,"id":"call_2","type":"function","function":{"name":"get_time","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"SF\"}"}},{"index":1,"function":{"arguments":"{\"tz\":\"PST\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"data: [DONE]",
	}, "\n") + "\n"

	body := io.NopCloser(strings.NewReader(sseData))
	ch := make(chan cobot.ProviderChunk, 64)

	p := &Provider{}
	go func() {
		p.readStream(body, ch)
	}()

	var toolCallChunks []cobot.ProviderChunk
	for c := range ch {
		if c.ToolCall != nil {
			toolCallChunks = append(toolCallChunks, c)
		}
	}

	if len(toolCallChunks) != 2 {
		t.Fatalf("expected 2 assembled tool calls, got %d", len(toolCallChunks))
	}

	tc0 := toolCallChunks[0].ToolCall
	if tc0.ID != "call_1" || tc0.Name != "get_weather" {
		t.Errorf("first tool call: expected call_1/get_weather, got %s/%s", tc0.ID, tc0.Name)
	}
	if string(tc0.Arguments) != `{"city":"SF"}` {
		t.Errorf("first tool call arguments: got %s", string(tc0.Arguments))
	}

	tc1 := toolCallChunks[1].ToolCall
	if tc1.ID != "call_2" || tc1.Name != "get_time" {
		t.Errorf("second tool call: expected call_2/get_time, got %s/%s", tc1.ID, tc1.Name)
	}
	if string(tc1.Arguments) != `{"tz":"PST"}` {
		t.Errorf("second tool call arguments: got %s", string(tc1.Arguments))
	}
}

func TestReadStreamTextOnly(t *testing.T) {
	sseData := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"data: [DONE]",
	}, "\n") + "\n"

	body := io.NopCloser(strings.NewReader(sseData))
	ch := make(chan cobot.ProviderChunk, 64)

	p := &Provider{}
	go func() {
		p.readStream(body, ch)
	}()

	var content string
	var gotDone bool
	var toolCallCount int
	for c := range ch {
		content += c.Content
		if c.Done {
			gotDone = true
		}
		if c.ToolCall != nil {
			toolCallCount++
		}
	}

	if !gotDone {
		t.Error("expected Done chunk")
	}
	if content != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", content)
	}
	if toolCallCount != 0 {
		t.Errorf("expected 0 tool calls, got %d", toolCallCount)
	}
}
```

- [ ] **Step 6: Run all OpenAI tests**

Run: `go test ./internal/llm/openai/ -v`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/llm/openai/provider.go internal/llm/openai/stream_test.go
git commit -m "fix: assemble OpenAI streaming tool call deltas by index"
```

---

### Task 2: Agent Loop — Handle Assembled Tool Calls + Emit Events

**Files:**
- Modify: `internal/agent/loop.go:52-114`
- Modify: `internal/agent/loop_test.go`

**Background:** The `Stream` method currently appends every `chunk.ToolCall` as a separate entry. With Task 1, the provider now emits assembled tool calls. But the loop needs to: (1) collect them correctly, (2) emit `EventToolCall` for each, (3) handle `Done` correctly — only finalize when `Done` is true AND there are no tool calls.

- [ ] **Step 1: Write the failing test for streaming tool calls**

Add to `internal/agent/loop_test.go`:

```go
type mockStreamProvider struct {
	chunks [][]cobot.ProviderChunk
	calls  int
}

func (m *mockStreamProvider) Name() string { return "mock-stream" }
func (m *mockStreamProvider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	return &cobot.ProviderResponse{Content: "done", StopReason: cobot.StopEndTurn}, nil
}
func (m *mockStreamProvider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	ch := make(chan cobot.ProviderChunk, 16)
	var chunks []cobot.ProviderChunk
	if m.calls < len(m.chunks) {
		chunks = m.chunks[m.calls]
	}
	m.calls++
	go func() {
		defer close(ch)
		for _, c := range chunks {
			ch <- c
		}
	}()
	return ch, nil
}

func TestAgentStreamWithToolCall(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10})
	a.SetProvider(&mockStreamProvider{
		chunks: [][]cobot.ProviderChunk{
			{
				{Content: "Let me check "},
				{Content: "that."},
				{ToolCall: &cobot.ToolCall{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"msg":"hi"}`)}},
				{Done: true},
			},
			{
				{Content: "The echo says: hi"},
				{Done: true},
			},
		},
	})
	a.ToolRegistry().Register(&echoTool{})

	ch, err := a.Stream(context.Background(), "run echo")
	if err != nil {
		t.Fatal(err)
	}

	var events []cobot.Event
	for evt := range ch {
		events = append(events, evt)
	}

	var textContent string
	var toolCallEvents []cobot.Event
	var toolResultEvents []cobot.Event
	var gotDone bool
	for _, e := range events {
		switch e.Type {
		case cobot.EventText:
			textContent += e.Content
		case cobot.EventToolCall:
			toolCallEvents = append(toolCallEvents, e)
		case cobot.EventToolResult:
			toolResultEvents = append(toolResultEvents, e)
		case cobot.EventDone:
			gotDone = true
		}
	}

	if !gotDone {
		t.Error("expected EventDone")
	}
	if textContent != "Let me check that.The echo says: hi" {
		t.Errorf("unexpected text content: %q", textContent)
	}
	if len(toolCallEvents) != 1 {
		t.Fatalf("expected 1 EventToolCall, got %d", len(toolCallEvents))
	}
	if toolCallEvents[0].ToolCall.ID != "call_1" {
		t.Errorf("expected tool call ID call_1, got %s", toolCallEvents[0].ToolCall.ID)
	}
	if len(toolResultEvents) != 1 {
		t.Fatalf("expected 1 EventToolResult, got %d", len(toolResultEvents))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestAgentStreamWithToolCall -v`
Expected: FAIL — `EventToolCall` is never emitted.

- [ ] **Step 3: Fix the `Stream` method**

Replace `internal/agent/loop.go:52-114` with:

```go
func (a *Agent) Stream(ctx context.Context, message string) (<-chan cobot.Event, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}

	ch := make(chan cobot.Event, 64)

	go func() {
		defer close(ch)
		a.session.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

		for turn := 0; turn < a.config.MaxTurns; turn++ {
			msgs := a.buildMessages(ctx)
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
				select {
				case <-ctx.Done():
					ch <- cobot.Event{Type: cobot.EventError, Error: ctx.Err()}
					return
				default:
				}
				if chunk.Content != "" {
					content += chunk.Content
					ch <- cobot.Event{Type: cobot.EventText, Content: chunk.Content}
				}
				if chunk.ToolCall != nil {
					toolCalls = append(toolCalls, *chunk.ToolCall)
					ch <- cobot.Event{Type: cobot.EventToolCall, ToolCall: chunk.ToolCall}
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

Key changes:
- Line `ch <- cobot.Event{Type: cobot.EventToolCall, ToolCall: chunk.ToolCall}` — now emits `EventToolCall` for each assembled tool call
- `Done` check only triggers finalization when there are zero tool calls (i.e., text-only response)
- When tool calls exist, the loop continues to next turn after executing them

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run TestAgentStreamWithToolCall -v`
Expected: PASS

- [ ] **Step 5: Run all agent tests**

Run: `go test ./internal/agent/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/loop.go internal/agent/loop_test.go
git commit -m "feat: emit EventToolCall during streaming, fix assembly logic"
```

---

### Task 3: TUI — Real-time Streaming with Tool Event Display

**Files:**
- Modify: `cmd/cobot/tui.go`

**Background:** The TUI currently uses a single blocking `tea.Cmd` that reads the entire stream before returning a `streamMsg`. Bubbletea only processes the returned `tea.Msg` after the function completes, so the UI never updates during streaming. The fix: spawn a goroutine that reads from the event channel and sends each event as a separate `streamMsg` via `p.Send()`, so the `Update` method is called for each chunk in real time.

- [ ] **Step 1: Rewrite the TUI**

Replace the entire `cmd/cobot/tui.go` with:

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type tuiModel struct {
	input     textinput.Model
	messages  []string
	agent     *agent.Agent
	streaming bool
	cancelFn  context.CancelFunc
	width     int
	height    int
	program   *tea.Program
}

type streamMsg struct {
	content    string
	eventType  cobot.EventType
	toolName   string
	done       bool
	err        error
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
				m.messages = append(m.messages, "")
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			if text == "/quit" || text == "/exit" {
				return m, tea.Quit
			}
			m.messages = append(m.messages, fmt.Sprintf("> %s", text))
			m.input.SetValue("")
			m.streaming = true
			return m, m.startStream(text)
		}

	case streamMsg:
		return m.handleStreamMsg(msg)
	}

	if m.streaming {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *tuiModel) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
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

	switch msg.eventType {
	case cobot.EventText:
		if len(m.messages) > 0 {
			last := m.messages[len(m.messages)-1]
			if strings.HasPrefix(last, "Assistant:") {
				m.messages[len(m.messages)-1] += msg.content
			} else {
				m.messages = append(m.messages, "Assistant: "+msg.content)
			}
		} else {
			m.messages = append(m.messages, "Assistant: "+msg.content)
		}
	case cobot.EventToolCall:
		label := fmt.Sprintf("  [Tool: %s]", msg.toolName)
		m.messages = append(m.messages, label)
	case cobot.EventToolResult:
		short := msg.content
		if len(short) > 200 {
			short = short[:200] + "..."
		}
		m.messages = append(m.messages, fmt.Sprintf("  [Result: %s]", short))
	}
	return m, nil
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
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel

	ch, err := m.agent.Stream(ctx, text)
	if err != nil {
		cancel()
		return func() tea.Msg { return streamMsg{err: err} }
	}

	p := m.program
	go func() {
		for evt := range ch {
			if p != nil {
				sm := streamMsg{
					content:   evt.Content,
					eventType: evt.Type,
					done:      evt.Done,
					err:       evt.Error,
				}
				if evt.ToolCall != nil {
					sm.toolName = evt.ToolCall.Name
				}
				p.Send(sm)
			}
		}
	}()

	return nil
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start interactive TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		a, cleanup, err := initAgent(cfg, false)
		if err != nil {
			return err
		}
		defer cleanup()

		model := newTUIModel(a)
		p := tea.NewProgram(model, tea.WithAltScreen())
		model.program = p
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
```

Key changes:
1. **`startStream`** no longer returns a blocking `tea.Cmd`. Instead it spawns a goroutine that reads from the event channel and calls `p.Send(streamMsg{...})` for each event. Bubbletea's `Update` is called for each `Send`, enabling real-time rendering.
2. **`streamMsg`** now includes `eventType` and `toolName` fields for rich event handling.
3. **`handleStreamMsg`** is extracted as a method to handle all event types: `EventText` (append to assistant message), `EventToolCall` (show tool name), `EventToolResult` (show truncated result).
4. **`program` field** on `tuiModel` gives the goroutine access to `tea.Program.Send()`.
5. `startStream` returns `nil` as `tea.Cmd` since the goroutine handles event delivery directly.

- [ ] **Step 2: Run build to verify compilation**

Run: `go build ./cmd/cobot/`
Expected: Success

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/cobot/tui.go
git commit -m "feat: real-time TUI streaming with tool event display"
```

---

### Task 4: Final Verification

**Files:** None

- [ ] **Step 1: Run full build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 2: Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 3: Run vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 4: Run race detector on streaming tests**

Run: `go test -race ./internal/llm/openai/ ./internal/agent/ -v`
Expected: All PASS, no races
