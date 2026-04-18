package main

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// notificationMsg is a BubbleTea message carrying a cron notification.
type notificationMsg struct {
	content string
}

// notificationShutdownMsg is sent when the notification channel is closed,
// signalling BubbleTea to stop polling.
type notificationShutdownMsg struct{}

// tuiChannel implements cobot.Channel for the TUI.
type tuiChannel struct {
	id     string
	alive  bool
	mu     sync.RWMutex
	notify chan<- cobot.ChannelMessage
	done   chan struct{} // closed in Close to unblock pollNotifications
}

func newTUIChannel(id string, notify chan<- cobot.ChannelMessage) *tuiChannel {
	return &tuiChannel{
		id:     id,
		alive:  true,
		notify: notify,
		done:   make(chan struct{}),
	}
}

func (ch *tuiChannel) ID() string { return ch.id }

func (ch *tuiChannel) Send(ctx context.Context, msg cobot.ChannelMessage) error {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if !ch.alive {
		return context.Canceled
	}
	select {
	case ch.notify <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ch *tuiChannel) IsAlive() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.alive
}

func (ch *tuiChannel) Close() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.alive {
		ch.alive = false
		close(ch.done) // unblock pollNotifications goroutine
		close(ch.notify)
	}
}

// Done returns a read-only channel that is closed by Close.
// Callers can select on it to detect shutdown.
func (ch *tuiChannel) Done() <-chan struct{} {
	return ch.done
}

// pollNotifications returns a tea.Cmd that waits for channel messages
// and converts them to notificationMsg for the BubbleTea Update loop.
// When the notify channel is closed, it returns notificationShutdownMsg
// to cleanly stop the polling cycle.
func pollNotifications(notify <-chan cobot.ChannelMessage, done <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-notify:
			if !ok {
				return notificationShutdownMsg{}
			}
			return notificationMsg{content: msg.Content}
		case <-done:
			return notificationShutdownMsg{}
		}
	}
}
