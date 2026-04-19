package broker

import (
	"context"
	"io"
	"time"
)

// Broker is the abstraction layer for distributed coordination.
// Implementations: SQLiteBroker (built-in), RedisBroker (optional).
// Consumers: cron scheduler (leader lease, notifications), channel manager (session registry, cross-process notifications).
// Future extensions: distributed task queue (Celery-style worker slices).
type Broker interface {
	Lock
	SessionRegistry
	PubSub
	io.Closer
}

// --- Lock: distributed lock / leader lease ---

type Lock interface {
	// TryAcquire attempts to acquire a distributed lock named name.
	// holder: unique identifier of the holder
	// ttl: lock lifetime, automatically released after timeout
	// Returns true if successfully acquired.
	TryAcquire(ctx context.Context, name, holder string, ttl time.Duration) (bool, error)

	// Renew extends the lease of an already held lock.
	Renew(ctx context.Context, name, holder string, ttl time.Duration) error

	// Release actively releases the lock.
	Release(ctx context.Context, name, holder string) error
}

// --- SessionRegistry: online instance/session tracking ---

// SessionInfo describes an active cobot instance/session.
type SessionInfo struct {
	ID        string // unique instance identifier
	ChannelID string // bound channel (e.g., "tui:default", "weixin", etc.)
	PID       int    // process ID
	StartedAt time.Time
}

type SessionRegistry interface {
	// Register registers the current instance.
	Register(ctx context.Context, info *SessionInfo) error

	// Unregister removes the instance.
	Unregister(ctx context.Context, sessionID string) error

	// Heartbeat refreshes the instance heartbeat.
	Heartbeat(ctx context.Context, sessionID string) error

	// ListByChannel returns all active instances bound to the specified channelID.
	ListByChannel(ctx context.Context, channelID string) ([]*SessionInfo, error)

	// ListAll returns all active instances.
	ListAll(ctx context.Context) ([]*SessionInfo, error)
}

// --- PubSub: cross-process message publishing/subscribing ---

// Message is a message for cross-process delivery.
type Message struct {
	ID        string // unique message identifier
	Topic     string // message topic (e.g., "cron_result", "channel:notify", etc.)
	ChannelID string // target channel (for routing)
	Payload   []byte // message body (JSON)
	CreatedAt time.Time
}

type PubSub interface {
	// Publish publishes a message.
	Publish(ctx context.Context, msg *Message) error

	// Consume consumes unacknowledged messages for the specified session.
	// Returns a list of unacknowledged messages. The caller should call Ack to confirm processing.
	Consume(ctx context.Context, topic, channelID, sessionID string, limit int) ([]*Message, error)

	// Ack confirms that the message has been processed by the specified session.
	Ack(ctx context.Context, msgID, sessionID string) error
}

// --- Queue: distributed task queue (reserved for future Celery/Redis integration) ---

// Task defines an asynchronous task.
type Task struct {
	ID       string
	Type     string // task type (routed to different handlers)
	Queue    string // queue name
	Payload  []byte // task body (JSON)
	Retries  int    // number of retries already performed
	MaxRetry int    // maximum number of retries
}

type Queue interface {
	// Enqueue delivers a task to the queue.
	Enqueue(ctx context.Context, task *Task) error

	// Dequeue consumes one task from the specified queue.
	Dequeue(ctx context.Context, queue string) (*Task, error)

	// Complete marks the task as completed.
	Complete(ctx context.Context, taskID string) error

	// Fail marks the task as failed, triggering retry or dead-letter queue.
	Fail(ctx context.Context, taskID string, err error) error
}
