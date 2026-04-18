package cobot

import "context"

// ChannelMessage represents a notification to be delivered to a Channel.
type ChannelMessage struct {
	Type    string // "cron_result", "info", "warning"
	Title   string // short summary
	Content string // full content
}

// Channel is an abstract communication endpoint that can receive notifications.
// Implementations: TUI, WeChat, Feishu, etc.
type Channel interface {
	// ID returns a unique identifier for this channel (e.g., "tui:default").
	ID() string

	// Send delivers a message to this channel.
	// Should be non-blocking or have a short timeout.
	Send(ctx context.Context, msg ChannelMessage) error

	// IsAlive returns true if the channel is still connected.
	IsAlive() bool
}
