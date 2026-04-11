package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type subagentSpawnArgs struct {
	Task           string   `json:"task"`
	Model          string   `json:"model,omitempty"`
	Tools          []string `json:"tools,omitempty"`
	MaxTurns       int      `json:"max_turns,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

type SpawnTool struct {
	coordinator *Coordinator
}

func NewSpawnTool(c *Coordinator) *SpawnTool {
	return &SpawnTool{coordinator: c}
}

func (t *SpawnTool) Name() string { return "subagent_spawn" }
func (t *SpawnTool) Description() string {
	return "Spawn a sub-agent to handle a task independently"
}
func (t *SpawnTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"task":{"type":"string","description":"The task for the sub-agent"},"model":{"type":"string","description":"Model override (optional)"},"tools":{"type":"array","items":{"type":"string"},"description":"Subset of tool names (optional, empty=all)"},"max_turns":{"type":"integer","description":"Max tool-calling turns (default 5)"},"timeout_seconds":{"type":"integer","description":"Timeout in seconds (default 300)"}},"required":["task"]}`)
}

func (t *SpawnTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a subagentSpawnArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	timeout := 5 * time.Minute
	if a.TimeoutSeconds > 0 {
		timeout = time.Duration(a.TimeoutSeconds) * time.Second
	}
	maxTurns := a.MaxTurns
	if maxTurns == 0 {
		maxTurns = 5
	}

	sa, err := t.coordinator.Spawn(ctx, &Config{
		Task:      a.Task,
		Model:     a.Model,
		ToolNames: a.Tools,
		MaxTurns:  maxTurns,
		Timeout:   timeout,
	})
	if err != nil {
		return "", err
	}

	select {
	case <-sa.Done():
		r := sa.Result()
		if r.Error != "" {
			return fmt.Sprintf("Sub-agent %s failed: %s", r.ID, r.Error), nil
		}
		return fmt.Sprintf("Sub-agent %s completed (took %s, %d tool calls):\n%s", r.ID, r.Duration.Round(time.Millisecond), r.ToolCalls, r.Output), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

var _ cobot.Tool = (*SpawnTool)(nil)
