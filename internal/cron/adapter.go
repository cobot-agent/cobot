package cron

import (
	"context"

	"github.com/cobot-agent/cobot/internal/agent"
)

// AgentRunnerAdapter adapts an *agent.Agent (which implements cobot.SubAgent)
// to the AgentRunner interface by calling Prompt and returning the content.
type AgentRunnerAdapter struct {
	Agent *agent.Agent
}

func (r *AgentRunnerAdapter) Prompt(ctx context.Context, message string) (string, error) {
	resp, err := r.Agent.Prompt(ctx, message)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (r *AgentRunnerAdapter) SetModel(spec string) error {
	return r.Agent.SetModel(spec)
}

func (r *AgentRunnerAdapter) Close() error {
	return r.Agent.Close()
}
