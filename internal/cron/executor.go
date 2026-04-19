package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// AgentRunner is the interface for running a prompt through an agent.
type AgentRunner interface {
	Prompt(ctx context.Context, message string) (string, error)
	SetModel(spec string) error
	Close() error
}

// AgentExecutor executes cron jobs by running a sub-agent session.
// Results are stored in the per-job SQLite RunStore.
type AgentExecutor struct {
	NewAgent func() AgentRunner
	runStore *RunStore
}

// NewAgentExecutor creates an executor with the given agent factory.
func NewAgentExecutor(factory func() AgentRunner) *AgentExecutor {
	return &AgentExecutor{NewAgent: factory}
}

// WithRunStore sets the run store for persisting execution records.
func (e *AgentExecutor) WithRunStore(store *RunStore) *AgentExecutor {
	e.runStore = store
	return e
}

// ExecuteJob runs the job's prompt through a new agent and stores the result.
func (e *AgentExecutor) ExecuteJob(ctx context.Context, job *Job) (string, error) {
	start := time.Now()

	runner := e.NewAgent()
	defer runner.Close()

	if job.Model != "" {
		if err := runner.SetModel(job.Model); err != nil {
			return "", fmt.Errorf("set model for cron job %s: %w", job.ID, err)
		}
	}

	result, err := runner.Prompt(ctx, job.Prompt)
	duration := time.Since(start).Milliseconds()

	// Record the run (success or failure) in per-job database.
	if e.runStore != nil {
		runRecord := &RunRecord{
			ID:       uuid.New().String()[:8],
			JobID:    job.ID,
			RunAt:    start,
			Duration: duration,
			Result:   result,
		}
		if err != nil {
			runRecord.Error = err.Error()
		}
		if storeErr := e.runStore.StoreRun(runRecord); storeErr != nil {
			slog.Warn("failed to store cron run record",
				"job_id", job.ID, "error", storeErr)
		}
	}

	if err != nil {
		return "", fmt.Errorf("execute cron job %s: %w", job.ID, err)
	}
	return result, nil
}
