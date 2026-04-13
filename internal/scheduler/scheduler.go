package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/cobot-agent/cobot/internal/agent"
)

type Scheduler struct {
	agent *agent.Agent
	cron  *cron.Cron
	mu    sync.RWMutex
	ids   map[string]cron.EntryID
	tasks map[string]*Task
	ctx   context.Context
}

func New(ctx context.Context, a *agent.Agent) *Scheduler {
	return &Scheduler{
		agent: a,
		cron:  cron.New(cron.WithSeconds()),
		ids:   make(map[string]cron.EntryID),
		tasks: make(map[string]*Task),
		ctx:   ctx,
	}
}

func (s *Scheduler) Start() error {
	s.cron.Start()
	return nil
}

func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

func (s *Scheduler) AddTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ids[task.Name]; exists {
		return fmt.Errorf("task %q already exists", task.Name)
	}

	id, err := s.cron.AddFunc(task.Schedule, func() {
		slog.Info("scheduler: executing task", "name", task.Name)
		resp, err := s.agent.Prompt(s.ctx, task.Prompt)
		if err != nil {
			slog.Error("scheduler: task failed", "name", task.Name, "error", err)
			return
		}
		if task.Output == "memory" && s.agent.MemoryStore() != nil {
			s.agent.MemoryStore().Store(s.ctx, resp.Content, "scheduler", task.Name)
		}
		slog.Info("scheduler: task completed", "name", task.Name)
	})
	if err != nil {
		return fmt.Errorf("parse schedule %q: %w", task.Schedule, err)
	}

	s.ids[task.Name] = id
	s.tasks[task.Name] = task
	return nil
}

func (s *Scheduler) RemoveTask(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.ids[name]
	if !ok {
		return fmt.Errorf("task %q not found", name)
	}
	s.cron.Remove(id)
	delete(s.ids, name)
	delete(s.tasks, name)
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
