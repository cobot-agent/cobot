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

// STMStoreFunc stores content to a session's short-term memory.
// Matches cobot.ShortTermMemory.StoreShortTerm signature.
type STMStoreFunc func(ctx context.Context, sessionID, content, category string) (string, error)

// AgentExecutor executes cron jobs by running a sub-agent session.
// Results are stored in LTM memory so the main session can recall them,
// and optionally in the originating session's STM for immediate context.
type AgentExecutor struct {
	NewAgent    func() AgentRunner
	MemoryStore MemoryStoreFunc
	STMStore    STMStoreFunc
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

// WithSTMStore sets the STM store callback for recording results in the
// originating session's short-term memory.
func (e *AgentExecutor) WithSTMStore(store STMStoreFunc) *AgentExecutor {
	e.STMStore = store
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

	// Store result in the originating session's STM so the next turn
	// in that session can see the cron result via getSTMContext.
	if e.STMStore != nil && job.SessionID != "" {
		stmContent := fmt.Sprintf("Cron job %q (id=%s) result: %s",
			job.Name, job.ID, result)
		if err != nil {
			stmContent = fmt.Sprintf("Cron job %q (id=%s) failed: %s",
				job.Name, job.ID, err)
		}
		if _, stmErr := e.STMStore(ctx, job.SessionID, stmContent, "observation"); stmErr != nil {
			slog.Warn("failed to store cron result in STM",
				"job_id", job.ID, "session_id", job.SessionID, "error", stmErr)
		}
	}

	return result, nil
}
