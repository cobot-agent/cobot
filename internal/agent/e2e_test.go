package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cobot-agent/cobot/internal/tools/builtin"
	cobot "github.com/cobot-agent/cobot/pkg"
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
		t.Errorf("expected 2 messages, got %d", len(a.Session().Messages()))
	}
}

func TestE2EToolCallFlow(t *testing.T) {
	a := New(&cobot.Config{MaxTurns: 10, Model: "mock"})
	a.ToolRegistry().Register(builtin.NewShellExecTool())

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
	msgs := a.Session().Messages()
	if len(msgs) != 4 {
		t.Errorf("expected 4 messages, got %d", len(msgs))
	}
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
