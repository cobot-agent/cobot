package broker

import (
	"context"
	"time"
)

// Broker is the abstraction layer for distributed coordination.
// Implementations: SQLiteBroker (built-in), RedisBroker (optional).
// Consumers: cron scheduler (leader lease, notifications), channel manager (session registry, cross-process notifications).
// Future extensions: distributed task queue (Celery-style worker slices).
type Broker interface {
	// --- Lock (leader election) ---
	TryAcquire(ctx context.Context, name, holder string, ttl time.Duration) (bool, error)
	Renew(ctx context.Context, name, holder string, ttl time.Duration) error
	Release(ctx context.Context, name, holder string) error

	// --- Session Registry (process liveness) ---
	Register(ctx context.Context, info *SessionInfo) error
	Unregister(ctx context.Context, sessionID string) error
	Heartbeat(ctx context.Context, sessionID string) error
	ListByChannel(ctx context.Context, channelID string) ([]*SessionInfo, error)
	ListAll(ctx context.Context) ([]*SessionInfo, error)

	// --- PubSub (message delivery) ---
	Publish(ctx context.Context, msg *Message) error
	Consume(ctx context.Context, topic, channelID, sessionID string, limit int) ([]*Message, error)
	Ack(ctx context.Context, msgID, sessionID string) error

	// --- Lifecycle ---
	Cleanup(ctx context.Context) error
	Close() error
}

// SessionInfo describes an active cobot instance/session.
type SessionInfo struct {
	ID        string // unique instance identifier
	ChannelID string // bound channel (e.g., "tui:default", "weixin", etc.)
	PID       int    // process ID
	StartedAt time.Time
}

// Message is a message for cross-process delivery.
// Note: The ID field is populated by Consume results (set to the auto-incremented
// database row ID). When calling Publish, the ID field is ignored — the database
// assigns the ID automatically.
type Message struct {
	ID        string // unique message identifier (set by Consume; ignored by Publish)
	Topic     string // message topic (e.g., "cron_result", "channel:notify", etc.)
	ChannelID string // target channel (for routing)
	Payload   []byte // message body (JSON)
	CreatedAt time.Time
}
