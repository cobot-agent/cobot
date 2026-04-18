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

// tuiChannel implements cobot.Channel for the TUI.
type tuiChannel struct {
	id     string
	alive  bool
	mu     sync.RWMutex
	notify chan<- cobot.ChannelMessage
}

func newTUIChannel(id string, notify chan<- cobot.ChannelMessage) *tuiChannel {
	return &tuiChannel{
		id:     id,
		alive:  true,
		notify: notify,
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
	ch.alive = false
}

// pollNotifications returns a tea.Cmd that waits for channel messages
// and converts them to notificationMsg for the BubbleTea Update loop.
func pollNotifications(notify <-chan cobot.ChannelMessage) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-notify
		if !ok {
			return nil
		}
		return notificationMsg{content: msg.Content}
	}
}
