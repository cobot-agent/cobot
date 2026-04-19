package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cobot-agent/cobot/internal/cron"
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

// SendTo delivers a message to a specific channel.
func (m *Manager) SendTo(ctx context.Context, channelID string, msg cobot.ChannelMessage) error {
	ch, alive := m.Get(channelID)
	if !alive {
		return fmt.Errorf("channel %s not available", channelID)
	}
	return ch.Send(ctx, msg)
}

// AllAliveIDs returns the IDs of all alive channels.
func (m *Manager) AllAliveIDs() []string {
	m.mu.RLock()
	candidates := make([]cobot.Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		candidates = append(candidates, ch)
	}
	m.mu.RUnlock()
	var ids []string
	for _, ch := range candidates {
		if ch.IsAlive() {
			ids = append(ids, ch.ID())
		}
	}
	return ids
}

// CronNotifier implements cron.Notifier using the ChannelManager.
type CronNotifier struct {
	manager        *Manager
	sessionChecker func(sessionID string) bool
}

func NewCronNotifier(mgr *Manager, sessionChecker func(string) bool) *CronNotifier {
	return &CronNotifier{
		manager:        mgr,
		sessionChecker: sessionChecker,
	}
}

func (n *CronNotifier) Notify(ctx context.Context, job *cron.Job, result string, execErr error) {
	ch, alive := n.manager.Get(job.ChannelID)
	if !alive {
		return
	}

	msg := cobot.ChannelMessage{
		Type:  "cron_result",
		Title: fmt.Sprintf("Cron job %q completed", job.Name),
	}

	if execErr != nil {
		msg.Content = fmt.Sprintf("❌ Job %s failed: %v", job.Name, execErr)
	} else {
		msg.Content = fmt.Sprintf("✅ Job %s result:\n%s", job.Name, result)
	}

	if n.sessionChecker != nil && n.sessionChecker(job.SessionID) {
		msg.Type = "cron_result_session"
	}

	if err := ch.Send(ctx, msg); err != nil {
		slog.Warn("failed to deliver cron notification",
			"channel", job.ChannelID, "job_id", job.ID, "error", err)
	}
}
