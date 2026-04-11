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
