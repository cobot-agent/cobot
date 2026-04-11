package subagent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestSpawnToolName(t *testing.T) {
	cfg := &cobot.Config{Model: "test", MaxTurns: 1}
	parent := agent.New(cfg)
	coord := NewCoordinator(parent)
	tool := NewSpawnTool(coord)

	if tool.Name() != "subagent_spawn" {
		t.Errorf("expected subagent_spawn, got %s", tool.Name())
	}
}

func TestSpawnToolExecute(t *testing.T) {
	cfg := &cobot.Config{Model: "test", MaxTurns: 1}
	parent := agent.New(cfg)
	coord := NewCoordinator(parent)
	tool := NewSpawnTool(coord)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"test task"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestSpawnToolInvalidArgs(t *testing.T) {
	cfg := &cobot.Config{Model: "test", MaxTurns: 1}
	parent := agent.New(cfg)
	coord := NewCoordinator(parent)
	tool := NewSpawnTool(coord)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
