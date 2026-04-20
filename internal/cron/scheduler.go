package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/pkg/broker"
)

// Scheduler manages cron job lifecycle using robfig/cron.
type Scheduler struct {
	store        *Store
	cron         *cron.Cron
	executeFn    func(ctx context.Context, jobID, prompt, model string) (string, error)
	notifier     cobot.Notifier // optional notification handler
	mu           sync.Mutex
	jobs         map[string]cron.EntryID // jobID -> cron entry ID
	jobSchedules map[string]string       // jobID -> schedule string (for change detection)
	runStore     *RunStore

	broker         broker.Broker
	holderID       string // leader lease identity
	sessionID      string // broker consume session identity
	isLeader       atomic.Bool
	cleanupRunning atomic.Bool
	wg             sync.WaitGroup
	cancel         context.CancelFunc
}

const maxCronJobs = 100

const jobTimeout = 10 * time.Minute

const leaseTTL = 30 * time.Second
const leaseRenewInterval = 10 * time.Second
const consumeInterval = 5 * time.Second
const cleanupInterval = 60 * time.Second

const brokerOpTimeout = 5 * time.Second
const schedulerLeaseKey = "cron:scheduler"

// NewScheduler creates a new Scheduler with the given store, execute function, run store, broker and notifier.
func NewScheduler(store *Store, executeFn func(ctx context.Context, jobID, prompt, model string) (string, error), runStore *RunStore, br broker.Broker, notifier cobot.Notifier) *Scheduler {
	return &Scheduler{
		store:        store,
		runStore:     runStore,
		notifier:     notifier,
		cron:         cron.New(),
		executeFn:    executeFn,
		jobs:         make(map[string]cron.EntryID),
		jobSchedules: make(map[string]string),
		broker:       br,
		holderID:     uuid.NewString(),
		sessionID:    uuid.NewString(),
	}
}

// Start loads all active jobs from the store, attempts to acquire the leader
// lease, and starts the appropriate loops. Returns an error only if loading
// jobs from the store fails.
func (s *Scheduler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	jobs, err := s.store.ListReadOnly()
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}

	acquired, err := s.broker.TryAcquire(ctx, schedulerLeaseKey, s.holderID, leaseTTL)
	if err != nil {
		slog.Warn("failed to acquire scheduler lease", "error", err)
	}

	if acquired {
		s.isLeader.Store(true)
		slog.Info("acquired cron scheduler leader lease", "holder", s.holderID)
		for _, job := range jobs {
			if job.Status != StatusActive {
				continue
			}
			if err := s.scheduleJob(job); err != nil {
				slog.Warn("failed to schedule job on start",
					"job_id", job.ID, "error", err)
			}
		}
		s.cron.Start()
		s.wg.Add(1)
		go s.renewLeaseLoop(ctx)
		s.cleanupRunning.Store(true)
		s.wg.Add(1)
		go s.cleanupLoop(ctx)
	} else {
		s.isLeader.Store(false)
		slog.Info("running as cron scheduler follower", "holder", s.holderID)
	}

	s.wg.Add(1)
	go s.consumeLoop(ctx)
	// All wg.Add(1) calls above happen synchronously before their respective
	// goroutines start, ensuring Stop()'s wg.Wait() always observes all
	// counters. The renewLeaseLoop may add wg.Add(1) for the cleanupLoop on
	// re-acquisition, but that Add also precedes the go call.
	return nil
}

// Stop halts the cron scheduler, releases the leader lease, and closes the run store.
func (s *Scheduler) Stop() {
	// Stop cron first to prevent new job executions.
	if s.isLeader.Load() {
		cronCtx := s.cron.Stop()
		<-cronCtx.Done() // wait for in-flight jobs to finish
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), brokerOpTimeout)
	defer cancel()

	if s.isLeader.Load() {
		if err := s.broker.Release(ctx, schedulerLeaseKey, s.holderID); err != nil {
			slog.Warn("failed to release scheduler lease", "error", err)
		}
	}

	if s.runStore != nil {
		s.runStore.Close()
	}
}

// validateSchedule parses the schedule without mutating in-memory cron state.
// Returns an error if the cron expression or one-shot timestamp is invalid.
func validateSchedule(job *Job) error {
	if job.OneShot {
		t, err := time.Parse(time.RFC3339, job.Schedule)
		if err != nil {
			return fmt.Errorf("invalid timestamp %q: %w", job.Schedule, err)
		}
		if t.Before(time.Now()) {
			return fmt.Errorf("one-shot time %q is in the past", job.Schedule)
		}
	} else {
		if _, err := cron.ParseStandard(job.Schedule); err != nil {
			return fmt.Errorf("invalid cron expression %q: %w", job.Schedule, err)
		}
	}
	return nil
}

// AddJob creates a new cron entry and persists the job.
// Validates the schedule regardless of leadership so that invalid expressions
// are rejected early. Only schedules in-memory if this instance is the leader;
// followers just persist — the leader will pick it up via syncJobs.
func (s *Scheduler) AddJob(job *Job) error {
	ids, err := s.store.ListJobIDs()
	if err != nil {
		return fmt.Errorf("check job count: %w", err)
	}
	if len(ids) >= maxCronJobs {
		return fmt.Errorf("maximum number of cron jobs (%d) reached", maxCronJobs)
	}
	// Validate schedule upfront regardless of leadership.
	if err := validateSchedule(job); err != nil {
		return err
	}
	// Persist first so we never have an in-memory-only job.
	if err := s.store.Create(job); err != nil {
		return err
	}
	if s.isLeader.Load() {
		if err := s.scheduleJob(job); err != nil {
			slog.Warn("failed to schedule persisted job", "job_id", job.ID, "error", err)
			// Job is persisted; leader will pick it up via syncJobs.
		}
	}
	return nil
}

// RemoveJob removes a job from cron and deletes it from the store.
// readID is required — it's an opaque token from a prior list/get that proves
// the caller has seen the current state.
func (s *Scheduler) RemoveJob(readID string) error {
	id, token, err := ParseReadID(readID)
	if err != nil {
		return fmt.Errorf("invalid read_id: %w", err)
	}
	s.unscheduleJob(id)
	if err := s.store.Delete(id, token); err != nil {
		return err
	}
	s.CleanupJobDB(id)
	return nil
}

// PauseJob removes a job from cron but keeps it in the store as paused.
// readID is required to verify the caller has seen the current state.
func (s *Scheduler) PauseJob(readID string) error {
	id, token, err := ParseReadID(readID)
	if err != nil {
		return fmt.Errorf("invalid read_id: %w", err)
	}

	job, err := s.store.Read(id, token)
	if err != nil {
		return err
	}

	s.unscheduleJob(id)

	job.Status = StatusPaused
	return s.store.Update(job) // Update regenerates token
}

// ResumeJob re-adds a paused job to cron and sets its status to active.
// Only schedules locally if this instance is the leader.
// readID is required to verify the caller has seen the current state.
func (s *Scheduler) ResumeJob(readID string) error {
	id, token, err := ParseReadID(readID)
	if err != nil {
		return fmt.Errorf("invalid read_id: %w", err)
	}

	job, err := s.store.Read(id, token)
	if err != nil {
		return err
	}
	if job.Status != StatusPaused {
		return fmt.Errorf("job %s is not paused (status: %s)", id, job.Status)
	}
	job.Status = StatusActive
	if s.isLeader.Load() {
		if err := s.scheduleJob(job); err != nil {
			return err
		}
	}
	return s.store.Update(job) // Update regenerates token
}

// ListJobs returns all jobs from the store.
func (s *Scheduler) ListJobs() ([]*Job, error) {
	return s.store.List()
}

// GetJob returns a single job by ID.
func (s *Scheduler) GetJob(id string) (*Job, error) {
	return s.store.Get(id)
}

// scheduleJob registers a job with the cron scheduler (acquires lock).
func (s *Scheduler) scheduleJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scheduleJobLocked(job)
}

// unscheduleJob removes a job from the in-memory cron scheduler (acquires lock).
func (s *Scheduler) unscheduleJob(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.jobs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, id)
		delete(s.jobSchedules, id)
	}
}

// scheduleJobLocked registers a job assuming mu is already held.
func (s *Scheduler) scheduleJobLocked(job *Job) error {
	// Remove existing entry if re-scheduling.
	if entryID, ok := s.jobs[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, job.ID)
		delete(s.jobSchedules, job.ID)
	}

	if job.OneShot {
		return s.scheduleOneShotLocked(job)
	}
	return s.scheduleCronExprLocked(job)
}

// scheduleCronExprLocked schedules a recurring job using a cron expression (caller holds mu).
func (s *Scheduler) scheduleCronExprLocked(job *Job) error {
	schedule, err := cron.ParseStandard(job.Schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", job.Schedule, err)
	}

	entryID := s.cron.Schedule(schedule, cron.FuncJob(func() {
		s.runJob(job)
	}))
	s.jobs[job.ID] = entryID
	s.jobSchedules[job.ID] = job.Schedule

	// Calculate and store next run time.
	next := s.cron.Entry(entryID).Next
	if !next.IsZero() {
		job.NextRun = &next
	}

	return nil
}

// scheduleOneShotLocked schedules a one-time job at a specific timestamp (caller holds mu).
func (s *Scheduler) scheduleOneShotLocked(job *Job) error {
	t, err := time.Parse(time.RFC3339, job.Schedule)
	if err != nil {
		return fmt.Errorf("invalid timestamp %q: %w", job.Schedule, err)
	}

	now := time.Now()
	if t.Before(now) {
		return fmt.Errorf("one-shot time %q is in the past", job.Schedule)
	}

	job.NextRun = &t

	// Verify the schedule will actually fire; if the time has passed between
	// the check above and scheduling (race window), fail instead of silently
	// succeeding without creating a cron entry.
	sched := oneShotSchedule{at: t}
	if next := sched.Next(time.Now()); next.IsZero() {
		return fmt.Errorf("one-shot time %q passed before scheduling", job.Schedule)
	}

	// Schedule with a custom one-shot wrapper.
	entryID := s.cron.Schedule(sched, cron.FuncJob(func() {
		s.runJob(job)
	}))
	s.jobs[job.ID] = entryID
	s.jobSchedules[job.ID] = job.Schedule

	return nil
}

// runJob executes a job and updates its last run info.
func (s *Scheduler) runJob(job *Job) {
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	start := time.Now()
	result, err := s.executeFn(ctx, job.ID, job.Prompt, job.Model)
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
}

// updateAndPersistJob updates job state (LastRun, RunCount, NextRun, Status)
// and persists the change, all under s.mu to avoid races with PauseJob/ResumeJob.
func (s *Scheduler) updateAndPersistJob(job *Job, now time.Time) {
	var toUpdate *Job
	s.mu.Lock()

	job.LastRun = &now
	job.RunCount++

	// stillScheduled guards against a stale *Job pointer captured by the cron
	// closure. If syncJobs (or RemoveJob) removed the entry from s.jobs after
	// the closure fired but before we acquired s.mu, the pointer is stale and
	// we must not persist its mutated fields. Without this check, we could
	// overwrite a job that was deleted or re-created with a different schedule.
	_, stillScheduled := s.jobs[job.ID]
	if !job.OneShot {
		if entryID, ok := s.jobs[job.ID]; ok {
			if next := s.cron.Entry(entryID).Next; !next.IsZero() {
				job.NextRun = &next
			}
		}
	} else {
		job.Status = StatusCompleted
		if entryID, ok := s.jobs[job.ID]; ok {
			s.cron.Remove(entryID)
			delete(s.jobs, job.ID)
			delete(s.jobSchedules, job.ID)
		}
	}

	// IMPORTANT: stillScheduled must be captured BEFORE one-shot cleanup
	// (one-shot cleanup deletes from s.jobs map, so check first).
	if stillScheduled {
		clone := *job // shallow copy of the job with updated fields
		toUpdate = &clone
	}
	s.mu.Unlock()

	if toUpdate != nil {
		if err := s.store.Update(toUpdate); err != nil {
			slog.Warn("failed to persist job update", "job_id", job.ID, "error", err)
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
	return "cron_" + uuid.NewString()[:8]
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
