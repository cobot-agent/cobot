package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/cobot-agent/cobot/internal/agent"
)

// NewAgentExecutor returns a function that executes a cron job by running a
// sub-agent session. The returned function creates a fresh agent per call,
// optionally sets the model, runs the prompt, stores the result in runStore,
// and returns the content string.
func NewAgentExecutor(newAgent func() *agent.Agent, runStore *RunStore) func(ctx context.Context, jobID, prompt, model string) (string, error) {
	return func(ctx context.Context, jobID, prompt, model string) (string, error) {
		start := time.Now()

		runner := newAgent()
		defer runner.Close()

		if model != "" {
			if err := runner.SetModel(model); err != nil {
				return "", fmt.Errorf("set model for cron job %s: %w", jobID, err)
			}
		}

		resp, err := runner.Prompt(ctx, prompt)
		duration := time.Since(start).Milliseconds()

		var result string
		if err == nil {
			result = resp.Content
		}

		// Record the run (success or failure) in per-job database.
		if runStore != nil {
			runRecord := &RunRecord{
				ID:       uuid.NewString(),
				JobID:    jobID,
				RunAt:    start,
				Duration: duration,
				Result:   result,
			}
			if err != nil {
				runRecord.Error = err.Error()
			}
			if storeErr := runStore.StoreRun(runRecord); storeErr != nil {
				slog.Warn("failed to store cron run record",
					"job_id", jobID, "error", storeErr)
			}
		}

		if err != nil {
			return "", fmt.Errorf("execute cron job %s: %w", jobID, err)
		}
		return result, nil
	}
}
