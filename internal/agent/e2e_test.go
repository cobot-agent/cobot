package agent

import (
	"context"
	"encoding/json"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// mockTool is a minimal Tool implementation used in E2E tests to avoid
// importing internal/tools/builtin (which would create an import cycle via
// workspace_tools.go → agent.LoadAgentConfig).
type mockTool struct {
	name   string
	result string
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return "mock tool" }
func (m *mockTool) Parameters() json.RawMessage { return json.RawMessage(`{}`) }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return m.result, nil
}

func TestE2ESimpleConversation(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"}, newTestRegistry())
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
	if len(a.SessionMgr().Session().Messages()) != 2 {
		t.Errorf("expected 2 messages, got %d", len(a.SessionMgr().Session().Messages()))
	}
}

func TestE2EToolCallFlow(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"}, newTestRegistry())
	a.ToolRegistry().Register(&mockTool{name: "shell_exec", result: "hello"})

	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{
				ToolCalls: []cobot.ToolCall{{
					ID:        "call_1",
					Name:      "shell_exec",
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
	msgs := a.SessionMgr().Session().Messages()
	if len(msgs) != 4 {
		t.Errorf("expected 4 messages, got %d", len(msgs))
	}
}

func TestE2EStreaming(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"}, newTestRegistry())
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
