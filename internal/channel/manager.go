package channel

import (
	"context"
	"log/slog"
	"sync"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// Manager tracks active channels and routes messages to them.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]cobot.Channel
}

func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]cobot.Channel),
	}
}

// Register adds a channel to the manager.
func (m *Manager) Register(ch cobot.Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.ID()] = ch
}

// Unregister removes a channel from the manager.
func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
}

// Get returns a channel by ID and whether it exists and is alive.
func (m *Manager) Get(id string) (cobot.Channel, bool) {
	m.mu.RLock()
	ch, ok := m.channels[id]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return ch, ch.IsAlive()
}

// AllAliveIDs returns the IDs of all alive channels.
func (m *Manager) AllAliveIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for _, ch := range m.channels {
		if ch.IsAlive() {
			ids = append(ids, ch.ID())
		}
	}
	return ids
}

// Notify delivers a message to the specified channel (implements cobot.Notifier).
func (m *Manager) Notify(ctx context.Context, channelID string, msg cobot.ChannelMessage) {
	ch, alive := m.Get(channelID)
	if !alive {
		return
	}
	if err := ch.Send(ctx, msg); err != nil {
		slog.Warn("failed to deliver notification",
			"channel", channelID, "error", err)
	}
}
