package scheduler

import (
	"testing"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestNewScheduler(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(a)
	if s == nil {
		t.Fatal("expected scheduler")
	}
}

func TestAddTask(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(a)

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
}

func TestAddTaskDuplicate(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(a)

	s.AddTask(&Task{Name: "dup", Schedule: "0 0 * * * *", Prompt: "x"})
	err := s.AddTask(&Task{Name: "dup", Schedule: "0 0 * * * *", Prompt: "y"})
	if err == nil {
		t.Error("expected error for duplicate task")
	}
}

func TestRemoveTask(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(a)

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
	s := New(a)

	err := s.RemoveTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestAddTaskBadSchedule(t *testing.T) {
	a := agent.New(cobot.DefaultConfig())
	s := New(a)

	err := s.AddTask(&Task{Name: "bad", Schedule: "not-a-cron", Prompt: "x"})
	if err == nil {
		t.Error("expected error for bad cron spec")
	}
}
