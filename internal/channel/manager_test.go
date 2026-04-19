package channel

import (
	"context"
	"sync"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// mockChannel implements cobot.Channel for testing.
type mockChannel struct {
	id     string
	alive  bool
	mu     sync.RWMutex
	sent   []cobot.ChannelMessage
	closed bool
}

func (m *mockChannel) ID() string { return m.id }

func (m *mockChannel) Send(_ context.Context, msg cobot.ChannelMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.alive {
		return context.Canceled
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockChannel) IsAlive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alive
}

func (m *mockChannel) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive = false
	m.closed = true
}

func TestManagerRegisterAndGet(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{id: "test:1", alive: true}

	mgr.Register(ch)

	got, alive := mgr.Get("test:1")
	if !alive {
		t.Fatal("expected channel to be alive")
	}
	if got.ID() != "test:1" {
		t.Fatalf("expected ID test:1, got %s", got.ID())
	}
}

func TestManagerGetUnknown(t *testing.T) {
	mgr := NewManager()

	_, alive := mgr.Get("nonexistent")
	if alive {
		t.Fatal("expected no channel for unknown ID")
	}
}

func TestManagerUnregister(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{id: "test:1", alive: true}

	mgr.Register(ch)
	mgr.Unregister("test:1")

	_, alive := mgr.Get("test:1")
	if alive {
		t.Fatal("expected channel to be gone after unregister")
	}
}

func TestManagerGetDeadChannel(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{id: "test:1", alive: true}

	mgr.Register(ch)
	ch.Close()

	_, alive := mgr.Get("test:1")
	if alive {
		t.Fatal("expected dead channel to report not alive")
	}
}

func TestManagerSendTo(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{id: "test:1", alive: true}
	mgr.Register(ch)

	err := mgr.SendTo(context.Background(), "test:1", cobot.ChannelMessage{
		Type:    "cron_result",
		Content: "done",
	})
	if err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}
	if len(ch.sent) != 1 || ch.sent[0].Content != "done" {
		t.Fatalf("expected message delivered, got sent=%v", ch.sent)
	}
}

func TestManagerSendToDeadChannel(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{id: "test:1", alive: true}
	mgr.Register(ch)
	ch.Close()

	err := mgr.SendTo(context.Background(), "test:1", cobot.ChannelMessage{})
	if err == nil {
		t.Fatal("expected error sending to dead channel")
	}
}

func TestManagerAllAliveIDs(t *testing.T) {
	mgr := NewManager()

	// Empty manager
	if ids := mgr.AllAliveIDs(); len(ids) != 0 {
		t.Fatalf("expected empty, got %v", ids)
	}

	// With alive channels
	ch1 := &mockChannel{id: "test:1", alive: true}
	ch2 := &mockChannel{id: "test:2", alive: true}
	mgr.Register(ch1)
	mgr.Register(ch2)
	ids := mgr.AllAliveIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}

	// Dead channel
	ch1.Close()
	ids = mgr.AllAliveIDs()
	if len(ids) != 1 || ids[0] != "test:2" {
		t.Fatalf("expected [test:2], got %v", ids)
	}
}
