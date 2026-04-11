package skills

import (
	"context"
	"strings"
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
	if !strings.Contains(result, "step1") {
		t.Error("expected step1 in result")
	}
	if !strings.Contains(result, "step2") {
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
