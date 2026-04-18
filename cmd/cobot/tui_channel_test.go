package main

import (
	"context"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestTUIChannelSendReceive(t *testing.T) {
	notify := make(chan cobot.ChannelMessage, 1)
	ch := newTUIChannel("tui:test", notify)

	if !ch.IsAlive() {
		t.Fatal("expected channel to be alive")
	}
	if ch.ID() != "tui:test" {
		t.Fatalf("expected ID tui:test, got %s", ch.ID())
	}

	msg := cobot.ChannelMessage{Type: "info", Content: "hello"}
	if err := ch.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	got := <-notify
	if got.Content != "hello" {
		t.Fatalf("expected content 'hello', got %q", got.Content)
	}
}

func TestTUIChannelSendAfterClose(t *testing.T) {
	notify := make(chan cobot.ChannelMessage, 1)
	ch := newTUIChannel("tui:test", notify)

	ch.Close()

	if ch.IsAlive() {
		t.Fatal("expected channel to be dead after Close")
	}

	err := ch.Send(context.Background(), cobot.ChannelMessage{})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestTUIChannelDoubleClose(t *testing.T) {
	notify := make(chan cobot.ChannelMessage, 1)
	ch := newTUIChannel("tui:test", notify)

	ch.Close()
	ch.Close() // must not panic

	if !isClosed(ch.Done()) {
		t.Fatal("expected Done channel to be closed")
	}
}

func TestTUIChannelDoneSignal(t *testing.T) {
	notify := make(chan cobot.ChannelMessage, 1)
	ch := newTUIChannel("tui:test", notify)

	if isClosed(ch.Done()) {
		t.Fatal("Done should not be closed before Close")
	}

	ch.Close()

	if !isClosed(ch.Done()) {
		t.Fatal("Done should be closed after Close")
	}
}

func TestPollNotificationsReceivesMessage(t *testing.T) {
	notify := make(chan cobot.ChannelMessage, 1)
	done := make(chan struct{})

	notify <- cobot.ChannelMessage{Content: "test-msg"}

	cmd := pollNotifications(notify, done)
	msg := cmd()

	nm, ok := msg.(notificationMsg)
	if !ok {
		t.Fatalf("expected notificationMsg, got %T", msg)
	}
	if nm.content != "test-msg" {
		t.Fatalf("expected content 'test-msg', got %q", nm.content)
	}
}

func TestPollNotificationsShutdownViaDone(t *testing.T) {
	notify := make(chan cobot.ChannelMessage)
	done := make(chan struct{})
	close(done)

	cmd := pollNotifications(notify, done)
	msg := cmd()

	if _, ok := msg.(notificationShutdownMsg); !ok {
		t.Fatalf("expected notificationShutdownMsg, got %T", msg)
	}
}

func TestPollNotificationsShutdownViaNotifyClose(t *testing.T) {
	notify := make(chan cobot.ChannelMessage)
	done := make(chan struct{})
	close(notify)

	cmd := pollNotifications(notify, done)
	msg := cmd()

	if _, ok := msg.(notificationShutdownMsg); !ok {
		t.Fatalf("expected notificationShutdownMsg, got %T", msg)
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
