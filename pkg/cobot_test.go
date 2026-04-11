package cobot

import (
	"context"
	"encoding/json"
	"testing"
)

type mockCore struct {
	provider Provider
	promptFn func(ctx context.Context, message string) (*ProviderResponse, error)
	streamFn func(ctx context.Context, message string) (<-chan Event, error)
	tools    []Tool
}

func (m *mockCore) SetProvider(p Provider) { m.provider = p }
func (m *mockCore) RegisterTool(t Tool)    { m.tools = append(m.tools, t) }
func (m *mockCore) Close() error           { return nil }
func (m *mockCore) Prompt(ctx context.Context, msg string) (*ProviderResponse, error) {
	return m.promptFn(ctx, msg)
}
func (m *mockCore) Stream(ctx context.Context, msg string) (<-chan Event, error) {
	return m.streamFn(ctx, msg)
}

func newTestCore(response string) *mockCore {
	return &mockCore{
		promptFn: func(ctx context.Context, msg string) (*ProviderResponse, error) {
			return &ProviderResponse{Content: response, StopReason: StopEndTurn}, nil
		},
		streamFn: func(ctx context.Context, msg string) (<-chan Event, error) {
			ch := make(chan Event, 2)
			ch <- Event{Type: EventText, Content: response}
			ch <- Event{Type: EventDone, Done: true}
			close(ch)
			return ch, nil
		},
	}
}

func TestNewAgent(t *testing.T) {
	cfg := DefaultConfig()
	core := newTestCore("")
	a, err := New(cfg, core)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestNewAgentNilConfig(t *testing.T) {
	core := newTestCore("")
	a, err := New(nil, core)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Error("expected non-nil agent with nil config")
	}
}

func TestAgentPrompt(t *testing.T) {
	core := newTestCore("Hello!")
	a, err := New(DefaultConfig(), core)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := a.Prompt(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("expected Hello!, got %s", resp.Content)
	}
}

func TestAgentStreaming(t *testing.T) {
	core := newTestCore("streamed")
	a, err := New(DefaultConfig(), core)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := a.Stream(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}

	var collected string
	for event := range ch {
		if event.Type == EventText {
			collected += event.Content
		}
	}
	if collected != "streamed" {
		t.Errorf("expected streamed, got %s", collected)
	}
}

func TestAgentSetProvider(t *testing.T) {
	core := newTestCore("")
	a, err := New(DefaultConfig(), core)
	if err != nil {
		t.Fatal(err)
	}

	tp := &testProvider{response: "test"}
	a.SetProvider(tp)
	if core.provider == nil {
		t.Error("expected provider to be set on core")
	}
}

func TestAgentRegisterTool(t *testing.T) {
	core := newTestCore("")
	a, err := New(DefaultConfig(), core)
	if err != nil {
		t.Fatal(err)
	}

	a.RegisterTool(&dummyTool{})
	if len(core.tools) != 1 {
		t.Errorf("expected 1 tool registered, got %d", len(core.tools))
	}
}

func TestAgentClose(t *testing.T) {
	core := newTestCore("")
	a, err := New(DefaultConfig(), core)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestAgentMemoryStore(t *testing.T) {
	core := newTestCore("")
	a, err := New(DefaultConfig(), core)
	if err != nil {
		t.Fatal(err)
	}
	if a.MemoryStore() != nil {
		t.Error("expected nil MemoryStore initially")
	}
	a.SetMemoryStore(nil)
	if a.MemoryStore() != nil {
		t.Error("expected nil")
	}
}

func TestAgentConfig(t *testing.T) {
	core := newTestCore("")
	cfg := DefaultConfig()
	a, err := New(cfg, core)
	if err != nil {
		t.Fatal(err)
	}
	if a.Config() != cfg {
		t.Error("expected same config pointer")
	}
}

func TestAgentProvider(t *testing.T) {
	core := newTestCore("")
	a, _ := New(DefaultConfig(), core)
	if a.Provider() != nil {
		t.Error("expected nil provider initially")
	}
}

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

type dummyTool struct{}

func (d *dummyTool) Name() string                { return "dummy" }
func (d *dummyTool) Description() string         { return "a dummy tool" }
func (d *dummyTool) Parameters() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (d *dummyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return "ok", nil
}
