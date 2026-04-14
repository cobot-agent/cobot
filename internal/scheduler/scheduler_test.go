package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestNewScheduler(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())
	if s == nil {
		t.Fatal("expected scheduler")
	}
}

func TestAddTask(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	err := s.AddTask(&Task{
		Name:     "test",
		Schedule: "0 0 * * * *",
		Prompt:   "test prompt",
	})
	if err != nil {
		t.Fatal(err)
	}

	tasks := s.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Name != "test" {
		t.Errorf("expected test, got %s", tasks[0].Name)
	}
	if !tasks[0].Enabled {
		t.Error("expected task to be enabled by default")
	}
}

func TestAddTaskDuplicate(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	s.AddTask(&Task{Name: "dup", Schedule: "0 0 * * * *", Prompt: "x"})
	err := s.AddTask(&Task{Name: "dup", Schedule: "0 0 * * * *", Prompt: "y"})
	if err == nil {
		t.Error("expected error for duplicate task")
	}
}

func TestRemoveTask(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	s.AddTask(&Task{Name: "remove-me", Schedule: "0 0 * * * *", Prompt: "x"})
	err := s.RemoveTask("remove-me")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ListTasks()) != 0 {
		t.Error("expected 0 tasks after remove")
	}
}

func TestRemoveTaskNotFound(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	err := s.RemoveTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestAddTaskBadSchedule(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	err := s.AddTask(&Task{Name: "bad", Schedule: "not-a-cron", Prompt: "x"})
	if err == nil {
		t.Error("expected error for bad cron spec")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	a := agent.New(cobot.DefaultConfig())

	// Create scheduler, add a task.
	s1 := New(context.Background(), a, dir)
	s1.AddTask(&Task{Name: "persist-test", Schedule: "0 0 * * * *", Prompt: "hello"})

	// Create new scheduler loading from same dir.
	s2 := New(context.Background(), a, dir)
	if err := s2.loadTasks(); err != nil {
		t.Fatal(err)
	}
	tasks := s2.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 persisted task, got %d", len(tasks))
	}
	if tasks[0].Name != "persist-test" {
		t.Errorf("expected persist-test, got %s", tasks[0].Name)
	}

	// Verify tasks.yaml exists.
	if _, err := os.Stat(filepath.Join(dir, "tasks.yaml")); err != nil {
		t.Errorf("tasks.yaml not created: %v", err)
	}
}

func TestEnableDisable(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	s.AddTask(&Task{Name: "toggle", Schedule: "0 0 * * * *", Prompt: "x"})

	if err := s.DisableTask("toggle"); err != nil {
		t.Fatal(err)
	}
	tasks := s.ListTasks()
	if tasks[0].Enabled {
		t.Error("expected task to be disabled")
	}

	if err := s.EnableTask("toggle"); err != nil {
		t.Fatal(err)
	}
	tasks = s.ListTasks()
	if !tasks[0].Enabled {
		t.Error("expected task to be enabled")
	}
}

func TestHistory(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	s.recordResult(TaskResult{Name: "t1", Success: true})
	s.recordResult(TaskResult{Name: "t2", Success: false, Error: "fail"})

	h := s.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(h))
	}
	if !h[0].Success || h[1].Success {
		t.Error("unexpected history results")
	}
}

func TestHistoryLimit(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(context.Background(), a, t.TempDir())

	for i := 0; i < 150; i++ {
		s.recordResult(TaskResult{Name: "t", Success: true})
	}
	h := s.History()
	if len(h) != 100 {
		t.Fatalf("expected 100 history entries (limited), got %d", len(h))
	}
}
