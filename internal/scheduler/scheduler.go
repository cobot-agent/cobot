package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/util"
)

const (
	tasksFile = "tasks.yaml"
)

type Scheduler struct {
	agent   *agent.Agent
	cron    *cron.Cron
	mu      sync.RWMutex
	ids     map[string]cron.EntryID
	tasks   map[string]*Task
	history []TaskResult
	dir     string // persistence directory
	ctx     context.Context
}

func New(ctx context.Context, a *agent.Agent, schedulerDir string) *Scheduler {
	return &Scheduler{
		agent: a,
		cron:  cron.New(cron.WithSeconds()),
		ids:   make(map[string]cron.EntryID),
		tasks: make(map[string]*Task),
		dir:   schedulerDir,
		ctx:   ctx,
	}
}

func (s *Scheduler) Start() error {
	if err := s.loadTasks(); err != nil {
		slog.Warn("scheduler: failed to load persisted tasks", "error", err)
	}
	// Register all persisted & enabled tasks under a write lock
	// (registerCron writes to s.ids).
	s.mu.Lock()
	for _, task := range s.tasks {
		if task.Enabled {
			if _, alreadyRegistered := s.ids[task.Name]; alreadyRegistered {
				continue
			}
			if err := s.registerCron(task); err != nil {
				slog.Error("scheduler: failed to register persisted task", "name", task.Name, "error", err)
			}
		}
	}
	s.mu.Unlock()

	s.cron.Start()
	return nil
}

func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

func (s *Scheduler) AddTask(task *Task) error {
	s.mu.Lock()

	if _, exists := s.ids[task.Name]; exists {
		s.mu.Unlock()
		return fmt.Errorf("task %q already exists", task.Name)
	}
	if _, exists := s.tasks[task.Name]; exists {
		s.mu.Unlock()
		return fmt.Errorf("task %q already exists", task.Name)
	}

	task.Enabled = true

	if err := s.registerCron(task); err != nil {
		s.mu.Unlock()
		return err
	}

	s.tasks[task.Name] = task

	tasks := s.snapshotTasks()
	s.mu.Unlock()

	if err := s.persistTasks(tasks); err != nil {
		slog.Error("scheduler: failed to persist tasks", "error", err)
	}
	return nil
}

func (s *Scheduler) RemoveTask(name string) error {
	s.mu.Lock()

	if _, exists := s.tasks[name]; !exists {
		s.mu.Unlock()
		return fmt.Errorf("task %q not found", name)
	}
	if id, hasID := s.ids[name]; hasID {
		s.cron.Remove(id)
		delete(s.ids, name)
	}
	delete(s.tasks, name)

	tasks := s.snapshotTasks()
	s.mu.Unlock()

	if err := s.persistTasks(tasks); err != nil {
		slog.Error("scheduler: failed to persist tasks", "error", err)
	}
	return nil
}

func (s *Scheduler) EnableTask(name string) error {
	s.mu.Lock()

	task, ok := s.tasks[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %q not found", name)
	}
	if task.Enabled {
		s.mu.Unlock()
		return nil
	}
	if err := s.registerCron(task); err != nil {
		s.mu.Unlock()
		return err
	}
	task.Enabled = true

	tasks := s.snapshotTasks()
	s.mu.Unlock()

	if err := s.persistTasks(tasks); err != nil {
		slog.Error("scheduler: failed to persist tasks", "error", err)
	}
	return nil
}

func (s *Scheduler) DisableTask(name string) error {
	s.mu.Lock()

	task, ok := s.tasks[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %q not found", name)
	}
	if !task.Enabled {
		s.mu.Unlock()
		return nil
	}
	if id, hasID := s.ids[name]; hasID {
		s.cron.Remove(id)
		delete(s.ids, name)
	}
	task.Enabled = false

	tasks := s.snapshotTasks()
	s.mu.Unlock()

	if err := s.persistTasks(tasks); err != nil {
		slog.Error("scheduler: failed to persist tasks", "error", err)
	}
	return nil
}

func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

func (s *Scheduler) History() []TaskResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TaskResult, len(s.history))
	copy(out, s.history)
	return out
}

// --- internal ---

func (s *Scheduler) registerCron(task *Task) error {
	id, err := s.cron.AddFunc(task.Schedule, func() {
		s.executeTask(task)
	})
	if err != nil {
		return fmt.Errorf("parse schedule %q: %w", task.Schedule, err)
	}
	s.ids[task.Name] = id
	return nil
}

func (s *Scheduler) executeTask(task *Task) {
	// Use a derived context with a 5-minute timeout per task execution.
	taskCtx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	startedAt := time.Now()
	slog.Info("scheduler: executing task", "name", task.Name)

	result := TaskResult{
		Name:      task.Name,
		StartedAt: startedAt.Format(time.RFC3339),
	}

	resp, err := s.agent.Prompt(taskCtx, task.Prompt)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		slog.Error("scheduler: task failed", "name", task.Name, "error", err)
	} else {
		result.Success = true
		slog.Info("scheduler: task completed", "name", task.Name)

		// Handle output.
		if task.Output == "memory" && s.agent.MemoryStore() != nil {
			if _, err := s.agent.MemoryStore().Store(taskCtx, resp.Content, "scheduler", task.Name); err != nil {
				slog.Error("scheduler: failed to store task output to memory", "name", task.Name, "error", err)
				result.Error = fmt.Sprintf("memory store: %v", err)
			}
		}
		if task.OutputPath != "" {
			if err := os.WriteFile(task.OutputPath, []byte(resp.Content), 0644); err != nil {
				slog.Error("scheduler: failed to write task output file", "name", task.Name, "path", task.OutputPath, "error", err)
				if result.Error == "" {
					result.Error = fmt.Sprintf("write file: %v", err)
				}
			}
		}
	}

	result.FinishedAt = time.Now().Format(time.RFC3339)
	s.recordResult(result)
}

func (s *Scheduler) recordResult(r TaskResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, r)
	if len(s.history) > util.HistoryLimit {
		s.history = s.history[len(s.history)-util.HistoryLimit:]
	}
}

// --- persistence ---

func (s *Scheduler) snapshotTasks() []*Task {
	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		cp := *t // shallow copy to avoid data race during persistence
		tasks = append(tasks, &cp)
	}
	return tasks
}

func (s *Scheduler) persistTasks(tasks []*Task) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create scheduler dir: %w", err)
	}
	data, err := yaml.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	path := filepath.Join(s.dir, tasksFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("persist tasks: %w", err)
	}
	return nil
}

func (s *Scheduler) loadTasks() error {
	if s.dir == "" {
		return nil
	}
	path := filepath.Join(s.dir, tasksFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read tasks file: %w", err)
	}
	var tasks []*Task
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("parse tasks file: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range tasks {
		s.tasks[t.Name] = t
	}
	slog.Info("scheduler: loaded persisted tasks", "count", len(tasks))
	return nil
}
