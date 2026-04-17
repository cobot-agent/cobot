package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"github.com/cobot-agent/cobot/internal/cron"
	cobot "github.com/cobot-agent/cobot/pkg"
)

//go:embed embed_cron_params.json
var cronParamsJSON []byte

var _ cobot.Tool = (*CronTool)(nil)

// CronTool allows the agent to schedule and manage recurring and one-shot tasks.
type CronTool struct {
	scheduler *cron.Scheduler
}

// NewCronTool creates a new CronTool with the given scheduler.
func NewCronTool(scheduler *cron.Scheduler) *CronTool {
	return &CronTool{scheduler: scheduler}
}

func (t *CronTool) Name() string { return "cron" }

func (t *CronTool) Description() string {
	return `Schedule and manage recurring and one-shot tasks. Actions: create (schedule a new job), list (show all jobs), delete (remove a job), pause (temporarily stop a job), resume (restart a paused job). Use cron expressions like "0 9 * * *" for recurring tasks or ISO timestamps like "2026-04-18T09:00:00Z" for one-shot tasks. Results are stored in memory and can be recalled with memory_search.`
}

func (t *CronTool) Parameters() json.RawMessage {
	return json.RawMessage(cronParamsJSON)
}

type cronParams struct {
	Action   string `json:"action"`
	Schedule string `json:"schedule"`
	Prompt   string `json:"prompt"`
	JobID    string `json:"job_id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
}

func (t *CronTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params cronParams
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	switch params.Action {
	case "create":
		return t.handleCreate(ctx, params)
	case "list":
		return t.handleList()
	case "delete":
		return t.handleDelete(params)
	case "pause":
		return t.handlePause(params)
	case "resume":
		return t.handleResume(params)
	default:
		return "", fmt.Errorf("unknown action: %s (valid: create, list, delete, pause, resume)", params.Action)
	}
}

func (t *CronTool) handleCreate(ctx context.Context, params cronParams) (string, error) {
	if params.Schedule == "" {
		return "", fmt.Errorf("schedule is required for create action")
	}
	if params.Prompt == "" {
		return "", fmt.Errorf("prompt is required for create action")
	}

	oneShot := cron.IsOneShot(params.Schedule)

	// Validate the schedule.
	if oneShot {
		if _, err := time.Parse(time.RFC3339, params.Schedule); err != nil {
			return "", fmt.Errorf("invalid ISO timestamp %q: %w", params.Schedule, err)
		}
	} else {
		if _, err := cronlib.ParseStandard(params.Schedule); err != nil {
			return "", fmt.Errorf("invalid cron expression %q: %w", params.Schedule, err)
		}
	}

	name := params.Name
	if name == "" {
		name = "unnamed"
	}

	job := &cron.Job{
		ID:        cron.NewJobID(),
		Name:      name,
		Schedule:  params.Schedule,
		Prompt:    params.Prompt,
		Model:     params.Model,
		Status:    "active",
		OneShot:   oneShot,
		CreatedAt: time.Now(),
	}

	if err := t.scheduler.AddJob(job); err != nil {
		return "", err
	}

	var nextStr string
	if job.NextRun != nil {
		nextStr = job.NextRun.Format(time.RFC3339)
	} else {
		nextStr = "N/A"
	}

	typ := "recurring"
	if oneShot {
		typ = "one-shot"
	}
	return fmt.Sprintf("Job created:\n  ID: %s\n  Name: %s\n  Schedule: %s\n  Type: %s\n  Next run: %s\n",
		job.ID, job.Name, job.Schedule, typ, nextStr), nil
}

func (t *CronTool) handleList() (string, error) {
	jobs, err := t.scheduler.ListJobs()
	if err != nil {
		return "", err
	}

	if len(jobs) == 0 {
		return "No cron jobs found.", nil
	}

	result := fmt.Sprintf("Cron jobs (%d):\n", len(jobs))
	for _, job := range jobs {
		lastRun := "never"
		if job.LastRun != nil {
			lastRun = job.LastRun.Format(time.RFC3339)
		}
		nextRun := "N/A"
		if job.NextRun != nil {
			nextRun = job.NextRun.Format(time.RFC3339)
		}
		result += fmt.Sprintf("  %s | %s | %s | status=%s | runs=%d | last=%s | next=%s\n",
			job.ID, job.Name, job.Schedule, job.Status, job.RunCount, lastRun, nextRun)
	}
	return result, nil
}

func (t *CronTool) handleDelete(params cronParams) (string, error) {
	if params.JobID == "" {
		return "", fmt.Errorf("job_id is required for delete action")
	}
	if err := t.scheduler.RemoveJob(params.JobID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Job %s deleted.", params.JobID), nil
}

func (t *CronTool) handlePause(params cronParams) (string, error) {
	if params.JobID == "" {
		return "", fmt.Errorf("job_id is required for pause action")
	}
	if err := t.scheduler.PauseJob(params.JobID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Job %s paused.", params.JobID), nil
}

func (t *CronTool) handleResume(params cronParams) (string, error) {
	if params.JobID == "" {
		return "", fmt.Errorf("job_id is required for resume action")
	}
	if err := t.scheduler.ResumeJob(params.JobID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Job %s resumed.", params.JobID), nil
}
