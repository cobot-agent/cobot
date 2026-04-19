package cobot

import (
	"context"
	"sync"
)

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

	// Close shuts down the channel, releasing resources.
	// It is safe to call Close multiple times.
	Close()
}

// BaseChannel provides common fields and methods for Channel implementations.
// Embed it in your struct and override Send() with your delivery logic.
type BaseChannel struct {
	id    string
	alive bool
	mu    sync.RWMutex
}

// NewBaseChannel creates a BaseChannel in alive state.
func NewBaseChannel(id string) BaseChannel {
	return BaseChannel{id: id, alive: true}
}

func (b *BaseChannel) ID() string { return b.id }

func (b *BaseChannel) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alive
}

// Close marks the channel as dead. Returns true if this is the first close
// (i.e. the channel was alive). Callers use this to decide whether to
// perform one-time cleanup of their own resources.
func (b *BaseChannel) Close() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.alive {
		return false
	}
	b.alive = false
	return true
}

// CheckAlive returns context.Canceled if the channel is dead, nil otherwise.
func (b *BaseChannel) CheckAlive() error {
	b.mu.RLock()
	alive := b.alive
	b.mu.RUnlock()
	if !alive {
		return context.Canceled
	}
	return nil
}

// WithRLock runs fn while holding the read lock. Use this to safely read
// your channel's own fields that are protected by BaseChannel's mutex.
func (b *BaseChannel) WithRLock(fn func()) {
	b.mu.RLock()
	fn()
	b.mu.RUnlock()
}

// WithLock runs fn while holding the write lock. Use this to safely modify
// your channel's own fields that are protected by BaseChannel's mutex.
func (b *BaseChannel) WithLock(fn func()) {
	b.mu.Lock()
	fn()
	b.mu.Unlock()
}
