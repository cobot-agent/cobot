package skills

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

func TestExecutorToolAndPromptBothSet(t *testing.T) {
	a := agent.New(&cobot.Config{Model: "mock", MaxTurns: 5})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{Content: "done", StopReason: cobot.StopEndTurn},
		},
	})

	skill := &Skill{
		Name:    "bothset",
		Trigger: "/bothset",
		Steps: []Step{
			{Tool: "some-tool", Prompt: "Do something"},
		},
	}

	exec := NewExecutor(a)
	_, err := exec.Execute(context.Background(), skill, "")
	if err == nil {
		t.Fatal("expected error when both tool and prompt are set")
	}
	if !strings.Contains(err.Error(), "cannot specify both tool") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecutorScriptToolFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script execution not supported on Windows")
	}
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(scriptsDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho \"hello from script\""), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scriptPath, 0755); err != nil {
		t.Fatal(err)
	}

	a := agent.New(&cobot.Config{Model: "mock", MaxTurns: 5})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{Content: "done", StopReason: cobot.StopEndTurn},
		},
	})

	skill := &Skill{
		Name:    "scripttest",
		Trigger: "/scripttest",
		Dir:     tmpDir,
		Steps: []Step{
			{Tool: "script", Args: map[string]any{"file": "test.sh"}, Output: "result"},
		},
	}

	exec := NewExecutor(a)
	result, err := exec.Execute(context.Background(), skill, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "hello from script") {
		t.Errorf("expected \"hello from script\" in result, got: %s", result)
	}
}

func TestExecutorScriptToolNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	a := agent.New(&cobot.Config{Model: "mock", MaxTurns: 5})
	a.SetProvider(&mockProvider{
		responses: []*cobot.ProviderResponse{
			{Content: "done", StopReason: cobot.StopEndTurn},
		},
	})

	skill := &Skill{
		Name:    "notfoundtest",
		Trigger: "/notfoundtest",
		Dir:     tmpDir,
		Steps: []Step{
			{Tool: "script", Args: map[string]any{"file": "missing.sh"}},
		},
	}

	exec := NewExecutor(a)
	_, err = exec.Execute(context.Background(), skill, "")
	if err == nil {
		t.Fatal("expected error for missing script file")
	}
	if !strings.Contains(err.Error(), "script file not found") {
		t.Errorf("unexpected error: %v", err)
	}
}
