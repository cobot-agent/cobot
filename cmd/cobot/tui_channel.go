package main

import (
	"context"

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
	cobot.BaseChannel
	notify chan<- cobot.ChannelMessage
	done   chan struct{} // closed in Close to unblock pollNotifications
}

func newTUIChannel(id string, notify chan<- cobot.ChannelMessage) *tuiChannel {
	return &tuiChannel{
		BaseChannel: cobot.NewBaseChannel(id),
		notify:      notify,
		done:        make(chan struct{}),
	}
}

func (ch *tuiChannel) Send(ctx context.Context, msg cobot.ChannelMessage) error {
	if err := ch.CheckAlive(); err != nil {
		return err
	}
	var notify chan<- cobot.ChannelMessage
	var done <-chan struct{}
	ch.WithRLock(func() {
		notify = ch.notify
		done = ch.done
	})
	select {
	case notify <- msg:
		return nil
	case <-done:
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ch *tuiChannel) Close() {
	if ch.BaseChannel.Close() {
		ch.WithLock(func() {
			close(ch.done)
			ch.notify = nil
		})
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
