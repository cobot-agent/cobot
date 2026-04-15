package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

//go:embed embed_delegate_task_params.json
var delegateTaskParamsJSON []byte

// SubAgentFactory creates a new SubAgent instance for delegation.
type SubAgentFactory func() cobot.SubAgent

type DelegateTool struct {
	factory SubAgentFactory
}

func NewDelegateTool(factory SubAgentFactory) *DelegateTool {
	return &DelegateTool{factory: factory}
}

func (t *DelegateTool) Name() string { return "delegate_task" }

func (t *DelegateTool) Description() string {
	return `Delegate a subtask to a sub-agent. The sub-agent runs autonomously and returns its result. Use for: complex subtasks, parallel work, isolated research. Parameters: prompt (required) - what the sub-agent should do; model (optional) - model spec like "openai:gpt-4o".`
}

func (t *DelegateTool) Parameters() json.RawMessage {
	return json.RawMessage(delegateTaskParamsJSON)
}

func (t *DelegateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}
	if params.Prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	sub := t.factory()
	if params.Model != "" {
		if err := sub.SetModel(params.Model); err != nil {
			return "", fmt.Errorf("set model: %w", err)
		}
	}

	resp, err := sub.Prompt(ctx, params.Prompt)
	if err != nil {
		return "", fmt.Errorf("sub-agent error: %w", err)
	}
	return resp.Content, nil
}
