package cron

import (
	"context"
	"fmt"

	"github.com/cobot-agent/cobot/internal/agent"
)

// NewAgentExecutor returns a function that executes a cron job by running a
// sub-agent session. The returned function creates a fresh agent per call,
// optionally sets the model, runs the prompt, and returns the content string.
func NewAgentExecutor(newAgent func() *agent.Agent) func(ctx context.Context, jobID, prompt, model string) (string, error) {
	return func(ctx context.Context, jobID, prompt, model string) (string, error) {
		runner := newAgent()
		defer runner.Close()

		if model != "" {
			if err := runner.SetModel(model); err != nil {
				return "", fmt.Errorf("set model for cron job %s: %w", jobID, err)
			}
		}

		resp, err := runner.Prompt(ctx, prompt)
		if err != nil {
			return "", fmt.Errorf("execute cron job %s: %w", jobID, err)
		}
		return resp.Content, nil
	}
}
