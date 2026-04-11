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
					ID:        "call_1",
					Name:      "echo",
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

func (e *echoTool) Name() string        { return "echo" }
func (e *echoTool) Description() string { return "echo back input" }
func (e *echoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`)
}
func (e *echoTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Msg string `json:"msg"`
	}
	json.Unmarshal(args, &a)
	return a.Msg, nil
}
