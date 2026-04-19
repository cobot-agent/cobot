package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/pkg/broker"
)

// JobExecutor is the interface for executing a cron job's prompt.
type JobExecutor interface {
	ExecuteJob(ctx context.Context, job *Job) (string, error)
}

// Scheduler manages cron job lifecycle using robfig/cron.
type Scheduler struct {
	store    *Store
	cron     *cron.Cron
	executor JobExecutor
	notifier cobot.Notifier // optional notification handler
	mu       sync.Mutex
	jobs     map[string]cron.EntryID // jobID -> cron entry ID
	runStore *RunStore

	broker   broker.Broker
	holderID string
	isLeader bool
}

const maxCronJobs = 100

const jobTimeout = 10 * time.Minute

const leaseTTL = 30 * time.Second
const leaseRenewInterval = 10 * time.Second
const consumeInterval = 5 * time.Second
const cleanupInterval = 60 * time.Second

// NewScheduler creates a new Scheduler with the given store, executor, run store, broker and notifier.
func NewScheduler(store *Store, executor JobExecutor, runStore *RunStore, br broker.Broker, notifier cobot.Notifier) *Scheduler {
	return &Scheduler{
		store:    store,
		runStore: runStore,
		notifier: notifier,
		cron:     cron.New(),
		executor: executor,
		jobs:     make(map[string]cron.EntryID),
		broker:   br,
		holderID: uuid.NewString(),
	}
}

// Start loads all active jobs from the store, attempts to acquire the leader
// lease, and starts the appropriate loops. Returns an error only if loading
// jobs from the store fails.
func (s *Scheduler) Start(ctx context.Context) error {
	jobs, err := s.store.List()
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}

	acquired, err := s.broker.TryAcquire(ctx, "cron:scheduler", s.holderID, leaseTTL)
	if err != nil {
		slog.Warn("failed to acquire scheduler lease", "error", err)
	}

	if acquired {
		s.isLeader = true
		slog.Info("acquired cron scheduler leader lease", "holder", s.holderID)
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
		go s.renewLeaseLoop(ctx)
		go s.cleanupLoop(ctx)
	} else {
		s.isLeader = false
		slog.Info("running as cron scheduler follower", "holder", s.holderID)
	}

	go s.consumeLoop(ctx)
	return nil
}

// Stop halts the cron scheduler, releases the leader lease, and closes the run store.
func (s *Scheduler) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.isLeader {
		cronCtx := s.cron.Stop()
		<-cronCtx.Done() // wait for in-flight jobs to finish
		if err := s.broker.Release(ctx, "cron:scheduler", s.holderID); err != nil {
			slog.Warn("failed to release scheduler lease", "error", err)
		}
	}

	if s.runStore != nil {
		s.runStore.Close()
	}

	if s.broker != nil {
		if err := s.broker.Close(); err != nil {
			slog.Warn("failed to close broker", "error", err)
		}
	}
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

	start := time.Now()
	result, err := s.executor.ExecuteJob(ctx, job)
	duration := time.Since(start)
	if err != nil {
		slog.Warn("cron job execution failed",
			"job_id", job.ID, "error", err)
	} else {
		slog.Debug("cron job executed",
			"job_id", job.ID, "result_len", len(result))
	}

	now := time.Now()
	s.updateAndPersistJob(job, now)

	s.publishJobResult(job, result, err, duration)
	s.notifyJobResult(job, result, err)
}

// updateAndPersistJob updates job state (LastRun, RunCount, NextRun, Status)
// and persists the change, all under s.mu to avoid races with PauseJob/ResumeJob.
func (s *Scheduler) updateAndPersistJob(job *Job, now time.Time) {
	job.LastRun = &now
	job.RunCount++

	s.mu.Lock()
	defer s.mu.Unlock()

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

	// Persist under lock to prevent races with PauseJob/ResumeJob.
	if !stillScheduled {
		slog.Debug("skipping update for removed job", "job_id", job.ID)
	} else if updateErr := s.store.Update(job); updateErr != nil {
		slog.Warn("failed to update job after run",
			"job_id", job.ID, "error", updateErr)
	}
}

// publishJobResult publishes the job result via the broker so followers can consume it.
func (s *Scheduler) publishJobResult(job *Job, result string, runErr error, duration time.Duration) {
	if s.broker == nil {
		return
	}
	payload := &broker.CronResultPayload{
		JobID:    job.ID,
		JobName:  job.Name,
		Result:   result,
		RunAt:    time.Now(),
		Duration: duration.Milliseconds(),
	}
	if runErr != nil {
		payload.Error = runErr.Error()
	}
	msg, err := broker.NewCronResultMessage(job.ChannelID, payload)
	if err != nil {
		slog.Warn("failed to build cron result message", "job_id", job.ID, "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.broker.Publish(ctx, msg); err != nil {
		slog.Warn("failed to publish cron result", "job_id", job.ID, "error", err)
	}
}

// notifyJobResult sends a notification about the job execution result.
func (s *Scheduler) notifyJobResult(job *Job, result string, err error) {
	if s.notifier == nil || job.ChannelID == "" {
		return
	}
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
	s.notifier.Notify(notifyCtx, job.ChannelID, msg)
	notifyCancel()
}

// renewLeaseLoop periodically renews the leader lease.
func (s *Scheduler) renewLeaseLoop(ctx context.Context) {
	ticker := time.NewTicker(leaseRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.broker.Renew(ctx, "cron:scheduler", s.holderID, leaseTTL); err != nil {
				slog.Warn("failed to renew scheduler lease", "error", err)
				return
			}
		}
	}
}

// cleanupLoop periodically runs broker cleanup (leader only).
func (s *Scheduler) cleanupLoop(ctx context.Context) {
	if cleanup, ok := s.broker.(interface {
		Cleanup(ctx context.Context) error
	}); ok {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := cleanup.Cleanup(ctx); err != nil {
					slog.Warn("broker cleanup failed", "error", err)
				}
			}
		}
	}
}

// consumeLoop periodically consumes cron result messages from the broker.
func (s *Scheduler) consumeLoop(ctx context.Context) {
	ticker := time.NewTicker(consumeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.consumeOnce(ctx)
		}
	}
}

// consumeOnce consumes unacknowledged cron result messages and delivers them locally.
func (s *Scheduler) consumeOnce(ctx context.Context) {
	if s.broker == nil || s.notifier == nil {
		return
	}
	// holderID is used as the sessionID for consumption.
	msgs, err := s.broker.Consume(ctx, "cron_result", "", s.holderID, 50)
	if err != nil {
		slog.Warn("failed to consume cron results", "error", err)
		return
	}
	for _, msg := range msgs {
		payload, err := broker.DecodeCronResult(msg)
		if err != nil {
			slog.Warn("failed to decode cron result", "msg_id", msg.ID, "error", err)
			_ = s.broker.Ack(ctx, msg.ID, s.holderID)
			continue
		}
		if msg.ChannelID == "" {
			_ = s.broker.Ack(ctx, msg.ID, s.holderID)
			continue
		}
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		var content string
		if payload.Error != "" {
			content = fmt.Sprintf("❌ Job %s failed: %s", payload.JobName, payload.Error)
		} else {
			content = fmt.Sprintf("✅ Job %s result:\n%s", payload.JobName, payload.Result)
		}
		s.notifier.Notify(notifyCtx, msg.ChannelID, cobot.ChannelMessage{
			Type:    cobot.MessageTypeCronResult,
			Title:   fmt.Sprintf("Cron job %q completed", payload.JobName),
			Content: content,
		})
		cancel()
		if ackErr := s.broker.Ack(ctx, msg.ID, s.holderID); ackErr != nil {
			slog.Warn("failed to ack cron result", "msg_id", msg.ID, "error", ackErr)
		}
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

// ListJobRuns returns execution records for a job.
func (s *Scheduler) ListJobRuns(jobID string, limit int) ([]*RunRecord, error) {
	if s.runStore == nil {
		return nil, nil
	}
	return s.runStore.ListRuns(jobID, limit)
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
