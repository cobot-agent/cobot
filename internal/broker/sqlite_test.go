package brokersqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cobot-agent/cobot/pkg/broker"
)

func tempBroker(t *testing.T) (*SQLiteBroker, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	b, err := NewSQLiteBroker(dbPath)
	if err != nil {
		t.Fatalf("new broker: %v", err)
	}
	return b, func() { _ = b.Close() }
}

func TestSQLiteBroker_Lock(t *testing.T) {
	b, cleanup := tempBroker(t)
	defer cleanup()
	ctx := context.Background()

	// First acquires the lock.
	ok, err := b.TryAcquire(ctx, "test", "A", 1*time.Second)
	if err != nil {
		t.Fatalf("A acquire: %v", err)
	}
	if !ok {
		t.Fatal("A should acquire lock")
	}

	// Second tries to acquire and should fail.
	ok2, err := b.TryAcquire(ctx, "test", "B", 1*time.Second)
	if err != nil {
		t.Fatalf("B acquire: %v", err)
	}
	if ok2 {
		t.Fatal("B should not acquire lock held by A")
	}

	// Renew.
	if err := b.Renew(ctx, "test", "A", 2*time.Second); err != nil {
		t.Fatalf("A renew: %v", err)
	}

	// Release.
	if err := b.Release(ctx, "test", "A"); err != nil {
		t.Fatalf("A release: %v", err)
	}

	// Second should now be able to acquire.
	ok3, err := b.TryAcquire(ctx, "test", "B", 1*time.Second)
	if err != nil {
		t.Fatalf("B second acquire: %v", err)
	}
	if !ok3 {
		t.Fatal("B should acquire lock after A released")
	}
}

func TestSQLiteBroker_LockExpire(t *testing.T) {
	b, cleanup := tempBroker(t)
	defer cleanup()
	ctx := context.Background()

	ok, _ := b.TryAcquire(ctx, "test", "A", 50*time.Millisecond)
	if !ok {
		t.Fatal("A should acquire")
	}

	time.Sleep(100 * time.Millisecond) // wait for lock to expire

	ok2, _ := b.TryAcquire(ctx, "test", "B", 1*time.Second)
	if !ok2 {
		t.Fatal("B should steal expired lock")
	}
}

func TestSQLiteBroker_PubSub(t *testing.T) {
	b, cleanup := tempBroker(t)
	defer cleanup()
	ctx := context.Background()

	msg := &broker.Message{
		ID:        "1",
		Topic:     "cron_result",
		ChannelID: "tui:default",
		Payload:   []byte(`{"job_id":"j1","result":"ok"}`),
		CreatedAt: time.Now(),
	}

	if err := b.Publish(ctx, msg); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Consume from session X's perspective.
	msgs, err := b.Consume(ctx, "cron_result", "tui:default", "session_X", 10)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if string(msgs[0].Payload) != `{"job_id":"j1","result":"ok"}` {
		t.Fatalf("unexpected payload: %s", msgs[0].Payload)
	}

	// Ack.
	if err := b.Ack(ctx, msgs[0].ID, "session_X"); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Should be empty after ack.
	msgs2, _ := b.Consume(ctx, "cron_result", "tui:default", "session_X", 10)
	if len(msgs2) != 0 {
		t.Fatalf("expected 0 after ack, got %d", len(msgs2))
	}
}

func TestSQLiteBroker_SessionRegistry(t *testing.T) {
	b, cleanup := tempBroker(t)
	defer cleanup()
	ctx := context.Background()

	s1 := &broker.SessionInfo{
		ID:        "s1",
		ChannelID: "tui:default",
		PID:       os.Getpid(),
		StartedAt: time.Now(),
	}
	if err := b.Register(ctx, s1); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Heartbeat.
	if err := b.Heartbeat(ctx, "s1"); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	sessions, err := b.ListByChannel(ctx, "tui:default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "s1" {
		t.Fatalf("expected s1, got %s", sessions[0].ID)
	}

	if err := b.Unregister(ctx, "s1"); err != nil {
		t.Fatalf("unregister: %v", err)
	}

	sessions2, _ := b.ListByChannel(ctx, "tui:default")
	if len(sessions2) != 0 {
		t.Fatalf("expected 0 after unregister, got %d", len(sessions2))
	}
}
