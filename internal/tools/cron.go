package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cobot-agent/cobot/internal/textutil"

	"github.com/cobot-agent/cobot/internal/cron"
	cobot "github.com/cobot-agent/cobot/pkg"
)

//go:embed schemas/embed_cron_params.json
var cronParamsJSON []byte

var _ cobot.Tool = (*CronTool)(nil)

// CronTool allows the agent to schedule and manage recurring and one-shot tasks.
type CronTool struct {
	scheduler   *cron.Scheduler
	channelIDFn func() string // returns the channel ID of the current context
}

// CronToolOption is a functional option for CronTool.
type CronToolOption func(*CronTool)

// WithCronChannelIDFn sets a function that returns the current channel ID.
// Cron job results are sent back to the originating channel.
func WithCronChannelIDFn(fn func() string) CronToolOption {
	return func(t *CronTool) { t.channelIDFn = fn }
}

// NewCronTool creates a new CronTool with the given scheduler.
func NewCronTool(scheduler *cron.Scheduler, opts ...CronToolOption) *CronTool {
	t := &CronTool{scheduler: scheduler}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *CronTool) Name() string { return "cron" }

// currentChannelID returns the channel ID from the injected function, or empty string.
// The default wiring (bootstrap.go) selects the first alive channel.
func (t *CronTool) currentChannelID() string {
	if t.channelIDFn != nil {
		return t.channelIDFn()
	}
	return ""
}

func (t *CronTool) Description() string {
	return `Schedule and manage recurring and one-shot tasks. Actions: create (schedule a new job), list (show all jobs), delete (remove a job), pause (temporarily stop a job), resume (restart a paused job), list_runs (show execution history for a job). Use cron expressions like "0 9 * * *" for recurring tasks or ISO timestamps for one-shot tasks. Results are stored in per-job run databases and can be viewed with list_runs.`
}

func (t *CronTool) Parameters() json.RawMessage {
	return json.RawMessage(cronParamsJSON)
}

type cronParams struct {
	Action   string `json:"action"`
	Schedule string `json:"schedule"`
	Prompt   string `json:"prompt"`
	JobID    string `json:"job_id"`
	ReadID   string `json:"read_id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	Limit    int    `json:"limit,omitempty"`
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
	case "list_runs":
		return t.handleListRuns(params)
	default:
		return "", fmt.Errorf("unknown action: %s (valid: create, list, delete, pause, resume, list_runs)", params.Action)
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
		ChannelID: t.currentChannelID(),
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
	return fmt.Sprintf("Job created:\n  ID: %s\n  read_id: %s\n  Name: %s\n  Schedule: %s\n  Type: %s\n  Next run: %s\n",
		job.ID, job.ReadID(), job.Name, job.Schedule, typ, nextStr), nil
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
		result += fmt.Sprintf("  %s | %s | %s | status=%s | runs=%d | last=%s | next=%s | read_id=%s\n",
			job.ID, job.Name, job.Schedule, job.Status, job.RunCount, lastRun, nextRun, job.ReadID())
	}
	return result, nil
}

func (t *CronTool) handleDelete(params cronParams) (string, error) {
	if params.ReadID == "" {
		return "", fmt.Errorf("read_id is required for delete action. Use the list action first to get the current read_id")
	}

	if err := t.scheduler.RemoveJob(params.ReadID); err != nil {
		return "", err
	}

	// Best-effort extraction for display; RemoveJob already validated the readID.
	jobID, _, _ := cron.ParseReadID(params.ReadID)
	return fmt.Sprintf("Job %s deleted.", jobID), nil
}

func (t *CronTool) handlePause(params cronParams) (string, error) {
	if params.ReadID == "" {
		return "", fmt.Errorf("read_id is required for pause action. Use the list action first to get the current read_id")
	}

	if err := t.scheduler.PauseJob(params.ReadID); err != nil {
		return "", err
	}

	// Best-effort extraction for display; PauseJob already validated the readID.
	jobID, _, _ := cron.ParseReadID(params.ReadID)
	return fmt.Sprintf("Job %s paused.", jobID), nil
}

func (t *CronTool) handleResume(params cronParams) (string, error) {
	if params.ReadID == "" {
		return "", fmt.Errorf("read_id is required for resume action. Use the list action first to get the current read_id")
	}

	if err := t.scheduler.ResumeJob(params.ReadID); err != nil {
		return "", err
	}

	// Best-effort extraction for display; ResumeJob already validated the readID.
	jobID, _, _ := cron.ParseReadID(params.ReadID)
	return fmt.Sprintf("Job %s resumed.", jobID), nil
}

func (t *CronTool) handleListRuns(params cronParams) (string, error) {
	if params.JobID == "" {
		return "", fmt.Errorf("job_id is required for list_runs action")
	}
	records, err := t.scheduler.ListJobRuns(params.JobID, params.Limit)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return fmt.Sprintf("No execution records for job %s.", params.JobID), nil
	}
	result := fmt.Sprintf("Execution records for job %s (%d most recent):\n", params.JobID, len(records))
	for _, r := range records {
		if r.Error != "" {
			result += fmt.Sprintf("  [%s] FAILED (%dms): %s\n", r.RunAt.Format("2006-01-02 15:04:05"), r.Duration, r.Error)
		} else {
			output := textutil.Truncate(r.Result, 100)
			result += fmt.Sprintf("  [%s] OK (%dms): %s\n", r.RunAt.Format("2006-01-02 15:04:05"), r.Duration, output)
		}
	}
	return result, nil
}
