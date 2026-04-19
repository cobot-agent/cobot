package channel

import (
	"context"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// mockChannel implements cobot.Channel for testing.
type mockChannel struct {
	cobot.BaseChannel
	sent   []cobot.ChannelMessage
	closed bool
}

func (m *mockChannel) Send(_ context.Context, msg cobot.ChannelMessage) error {
	if err := m.CheckAlive(); err != nil {
		return err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockChannel) Close() {
	if m.BaseChannel.Close() {
		m.closed = true
	}
}

func TestManagerRegisterAndGet(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{BaseChannel: cobot.NewBaseChannel("test:1")}

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
	ch := &mockChannel{BaseChannel: cobot.NewBaseChannel("test:1")}

	mgr.Register(ch)
	mgr.Unregister("test:1")

	_, alive := mgr.Get("test:1")
	if alive {
		t.Fatal("expected channel to be gone after unregister")
	}
}

func TestManagerGetDeadChannel(t *testing.T) {
	mgr := NewManager()
	ch := &mockChannel{BaseChannel: cobot.NewBaseChannel("test:1")}

	mgr.Register(ch)
	ch.Close()

	_, alive := mgr.Get("test:1")
	if alive {
		t.Fatal("expected dead channel to report not alive")
	}
}

func TestManagerAllAliveIDs(t *testing.T) {
	mgr := NewManager()

	// Empty manager
	if ids := mgr.AllAliveIDs(); len(ids) != 0 {
		t.Fatalf("expected empty, got %v", ids)
	}

	// With alive channels
	ch1 := &mockChannel{BaseChannel: cobot.NewBaseChannel("test:1")}
	ch2 := &mockChannel{BaseChannel: cobot.NewBaseChannel("test:2")}
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
