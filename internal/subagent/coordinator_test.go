package subagent

import (
	"context"
	"testing"
	"time"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestNewCoordinator(t *testing.T) {
	parent := agent.New(&cobot.Config{Model: "test-model"})
	coord := NewCoordinator(parent)
	if coord == nil {
		t.Fatal("expected non-nil coordinator")
	}
	if coord.parent != parent {
		t.Error("parent not set correctly")
	}
}

func TestSpawnNoProvider(t *testing.T) {
	parent := agent.New(&cobot.Config{Model: "test-model"})
	coord := NewCoordinator(parent)

	sa, err := coord.Spawn(context.Background(), &Config{
		Task:    "do something",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}
	if sa == nil {
		t.Fatal("expected non-nil subagent")
	}
	if sa.ID == "" {
		t.Error("expected non-empty ID")
	}

	<-sa.Done()

	r := sa.Result()
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if r.Error == "" {
		t.Error("expected error in result since no provider set")
	}
}

func TestGather(t *testing.T) {
	parent := agent.New(&cobot.Config{Model: "test-model"})
	coord := NewCoordinator(parent)

	sa, _ := coord.Spawn(context.Background(), &Config{
		Task:    "hello",
		Timeout: 2 * time.Second,
	})

	results := coord.Gather(context.Background(), []string{sa.ID})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != sa.ID {
		t.Errorf("expected ID %s, got %s", sa.ID, results[0].ID)
	}
	if results[0].Error == "" {
		t.Error("expected error since no provider configured")
	}
}

func TestGetNonexistent(t *testing.T) {
	parent := agent.New(&cobot.Config{Model: "test-model"})
	coord := NewCoordinator(parent)

	_, ok := coord.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent subagent")
	}
}

func TestCancelAll(t *testing.T) {
	parent := agent.New(&cobot.Config{Model: "test-model"})
	coord := NewCoordinator(parent)

	sa, _ := coord.Spawn(context.Background(), &Config{
		Task:    "long task",
		Timeout: 10 * time.Second,
	})

	coord.CancelAll()

	select {
	case <-sa.Done():
	case <-time.After(2 * time.Second):
		t.Error("subagent should have been cancelled")
	}
}
