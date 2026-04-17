package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// AgentRunner is the interface for running a prompt through an agent.
type AgentRunner interface {
	Prompt(ctx context.Context, message string) (string, error)
	SetModel(spec string) error
	Close() error
}

// AgentExecutor executes cron jobs by running a sub-agent session.
// Results are stored in LTM memory so the main session can recall them.
type AgentExecutor struct {
	NewAgent    func() AgentRunner
	MemoryStore MemoryStoreFunc
}

// MemoryStoreFunc stores content to LTM. Matches MemoryStore.StoreByName signature.
type MemoryStoreFunc func(ctx context.Context, content, wingName, roomName, hallType string) (string, error)

// NewAgentExecutor creates an executor with the given agent factory.
func NewAgentExecutor(factory func() AgentRunner) *AgentExecutor {
	return &AgentExecutor{NewAgent: factory}
}

// WithMemoryStore sets the memory store callback for persisting results.
func (e *AgentExecutor) WithMemoryStore(store MemoryStoreFunc) *AgentExecutor {
	e.MemoryStore = store
	return e
}

// ExecuteJob runs the job's prompt through a new agent and stores the result.
func (e *AgentExecutor) ExecuteJob(ctx context.Context, job *Job) (string, error) {
	runner := e.NewAgent()
	defer runner.Close()

	if job.Model != "" {
		if err := runner.SetModel(job.Model); err != nil {
			return "", fmt.Errorf("set model for cron job %s: %w", job.ID, err)
		}
	}

	result, err := runner.Prompt(ctx, job.Prompt)
	if err != nil {
		return "", fmt.Errorf("execute cron job %s: %w", job.ID, err)
	}

	// Store result in LTM memory if a store is configured.
	if e.MemoryStore != nil {
		content := fmt.Sprintf("[%s] Cron job %q (id=%s) executed:\n%s",
			time.Now().Format(time.RFC3339), job.Name, job.ID, result)
		if _, storeErr := e.MemoryStore(ctx, content, "cron",
			fmt.Sprintf("job_%s", job.ID), "drawer"); storeErr != nil {
			slog.Warn("failed to store cron result in memory",
				"job_id", job.ID, "error", storeErr)
		}
	}

	return result, nil
}
