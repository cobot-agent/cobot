package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/cobot-agent/cobot/internal/textutil"
	cobot "github.com/cobot-agent/cobot/pkg"
)

// JobExecutor is the interface for executing a cron job's prompt.
type JobExecutor interface {
	ExecuteJob(ctx context.Context, job *Job) (string, error)
}

// Notifier delivers messages to channels (generic, not cron-specific).
type Notifier = cobot.Notifier

// Scheduler manages cron job lifecycle using robfig/cron.
type Scheduler struct {
	store    *Store
	cron     *cron.Cron
	executor JobExecutor
	notifier Notifier     // optional notification handler
	nmu      sync.RWMutex // protects notifier
	mu       sync.Mutex
	jobs     map[string]cron.EntryID // jobID -> cron entry ID
	runStore *RunStore
}

const maxCronJobs = 100

const jobTimeout = 10 * time.Minute

// NewScheduler creates a new Scheduler with the given store, executor, and run store.
func NewScheduler(store *Store, executor JobExecutor, runStore *RunStore) *Scheduler {
	return &Scheduler{
		store:    store,
		runStore: runStore,
		cron:     cron.New(),
		executor: executor,
		jobs:     make(map[string]cron.EntryID),
	}
}

// SetNotifier sets the optional notifier for delivering job results.
func (s *Scheduler) SetNotifier(n Notifier) {
	s.nmu.Lock()
	s.notifier = n
	s.nmu.Unlock()
}

// getNotifier returns the current notifier under read lock.
func (s *Scheduler) getNotifier() Notifier {
	s.nmu.RLock()
	n := s.notifier
	s.nmu.RUnlock()
	return n
}

// Start loads all active jobs from the store and starts the cron scheduler.
func (s *Scheduler) Start() error {
	jobs, err := s.store.List()
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	for _, job := range jobs {
		if job.Status != "active" {
			continue
		}
		if err := s.scheduleJob(job); err != nil {
			slog.Warn("failed to schedule job on start",
				"job_id", job.ID, "error", err)
		}
	}
	s.cron.Start()
	return nil
}

// Stop halts the cron scheduler and removes all entries.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// AddJob creates a new cron entry and persists the job.
func (s *Scheduler) AddJob(job *Job) error {
	jobs, err := s.store.List()
	if err != nil {
		return fmt.Errorf("check job count: %w", err)
	}
	if len(jobs) >= maxCronJobs {
		return fmt.Errorf("maximum number of cron jobs (%d) reached", maxCronJobs)
	}
	if err := s.scheduleJob(job); err != nil {
		return err
	}
	return s.store.Create(job)
}

// RemoveJob removes a job from cron and deletes it from the store.
func (s *Scheduler) RemoveJob(id string) error {
	s.mu.Lock()
	if entryID, ok := s.jobs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, id)
	}
	s.mu.Unlock()
	if err := s.store.Delete(id); err != nil {
		return err
	}
	s.CleanupJobDB(id)
	return nil
}

// PauseJob removes a job from cron but keeps it in the store as paused.
func (s *Scheduler) PauseJob(id string) error {
	s.mu.Lock()
	if entryID, ok := s.jobs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, id)
	}
	s.mu.Unlock()

	job, err := s.store.Get(id)
	if err != nil {
		return err
	}
	job.Status = "paused"
	return s.store.Update(job)
}

// ResumeJob re-adds a paused job to cron and sets its status to active.
func (s *Scheduler) ResumeJob(id string) error {
	job, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if job.Status != "paused" {
		return fmt.Errorf("job %s is not paused (status: %s)", id, job.Status)
	}
	job.Status = "active"
	if err := s.scheduleJob(job); err != nil {
		return err
	}
	return s.store.Update(job)
}

// ListJobs returns all jobs from the store.
func (s *Scheduler) ListJobs() ([]*Job, error) {
	return s.store.List()
}

// GetJob returns a single job by ID.
func (s *Scheduler) GetJob(id string) (*Job, error) {
	return s.store.Get(id)
}

// scheduleJob registers a job with the cron scheduler.
func (s *Scheduler) scheduleJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entry if re-scheduling.
	if entryID, ok := s.jobs[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, job.ID)
	}

	if job.OneShot {
		return s.scheduleOneShot(job)
	}
	return s.scheduleCronExpr(job)
}

// scheduleCronExpr schedules a recurring job using a cron expression.
func (s *Scheduler) scheduleCronExpr(job *Job) error {
	schedule, err := cron.ParseStandard(job.Schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", job.Schedule, err)
	}

	entryID := s.cron.Schedule(schedule, cron.FuncJob(func() {
		s.runJob(job)
	}))
	s.jobs[job.ID] = entryID

	// Calculate and store next run time.
	next := s.cron.Entry(entryID).Next
	if !next.IsZero() {
		job.NextRun = &next
	}

	return nil
}

// scheduleOneShot schedules a one-time job at a specific timestamp.
func (s *Scheduler) scheduleOneShot(job *Job) error {
	t, err := time.Parse(time.RFC3339, job.Schedule)
	if err != nil {
		return fmt.Errorf("invalid timestamp %q: %w", job.Schedule, err)
	}

	now := time.Now()
	if t.Before(now) {
		return fmt.Errorf("one-shot time %q is in the past", job.Schedule)
	}

	job.NextRun = &t

	// Schedule with a custom one-shot wrapper.
	entryID := s.cron.Schedule(oneShotSchedule{at: t}, cron.FuncJob(func() {
		s.runJob(job)
	}))
	s.jobs[job.ID] = entryID

	return nil
}

// runJob executes a job and updates its last run info.
func (s *Scheduler) runJob(job *Job) {
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	result, err := s.executor.ExecuteJob(ctx, job)
	if err != nil {
		slog.Warn("cron job execution failed",
			"job_id", job.ID, "error", err)
	} else {
		slog.Debug("cron job executed",
			"job_id", job.ID, "result_len", len(result))
	}

	now := time.Now()
	job.LastRun = &now
	job.RunCount++

	// Snapshot existence under lock, update next-run or finalize one-shot.
	s.mu.Lock()
	_, stillScheduled := s.jobs[job.ID]
	if !job.OneShot {
		if entryID, ok := s.jobs[job.ID]; ok {
			if next := s.cron.Entry(entryID).Next; !next.IsZero() {
				job.NextRun = &next
			}
		}
	} else {
		job.Status = "completed"
		if entryID, ok := s.jobs[job.ID]; ok {
			s.cron.Remove(entryID)
			delete(s.jobs, job.ID)
		}
	}
	s.mu.Unlock()

	// Persist update outside lock. Skip if job was removed mid-execution.
	if !stillScheduled {
		slog.Debug("skipping update for removed job", "job_id", job.ID)
	} else {
		if updateErr := s.store.Update(job); updateErr != nil {
			slog.Warn("failed to update job after run",
				"job_id", job.ID, "error", updateErr)
		}
	}

	// Notify the originating channel if a notifier is configured.
	if n := s.getNotifier(); n != nil && job.ChannelID != "" {
		notifyCtx, notifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg := cobot.ChannelMessage{
			Type:  cobot.MessageTypeCronResult,
			Title: fmt.Sprintf("Cron job %q completed", job.Name),
		}
		if err != nil {
			msg.Content = fmt.Sprintf("❌ Job %s failed: %v", job.Name, err)
		} else {
			msg.Content = fmt.Sprintf("✅ Job %s result:\n%s", job.Name, result)
		}
		n.Notify(notifyCtx, job.ChannelID, msg)
		notifyCancel()
	}
}

// HasRunRecords checks if a job has execution history.
func (s *Scheduler) HasRunRecords(jobID string) (bool, error) {
	if s.runStore == nil {
		return false, nil
	}
	return s.runStore.RunsExist(jobID)
}

// CleanupJobDB removes the run database for a job.
func (s *Scheduler) CleanupJobDB(jobID string) {
	if s.runStore != nil {
		if err := s.runStore.DeleteJobDB(jobID); err != nil {
			slog.Warn("failed to delete run db", "job_id", jobID, "error", err)
		}
	}
}

// ListJobRuns returns formatted execution records for a job.
func (s *Scheduler) ListJobRuns(jobID string, limit int) (string, error) {
	if s.runStore == nil {
		return "Run tracking not available.", nil
	}
	records, err := s.runStore.ListRuns(jobID, limit)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return fmt.Sprintf("No execution records for job %s.", jobID), nil
	}
	result := fmt.Sprintf("Execution records for job %s (%d most recent):\n", jobID, len(records))
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

// NewJobID generates a friendly cron job ID.
func NewJobID() string {
	return "cron_" + uuid.New().String()[:8]
}

// IsOneShot detects if a schedule string is an ISO timestamp (one-shot).
func IsOneShot(schedule string) bool {
	_, err := time.Parse(time.RFC3339, schedule)
	return err == nil
}

// oneShotSchedule implements cron.Schedule for a single fire time.
type oneShotSchedule struct {
	at time.Time
}

func (o oneShotSchedule) Next(t time.Time) time.Time {
	if t.Before(o.at) {
		return o.at
	}
	return time.Time{} // zero = no more runs
}
