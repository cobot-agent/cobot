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

// CronNotifier implements cron.Notifier using the ChannelManager.
type CronNotifier struct {
	manager *Manager
}

func NewCronNotifier(mgr *Manager) *CronNotifier {
	return &CronNotifier{
		manager: mgr,
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

	if err := ch.Send(ctx, msg); err != nil {
		slog.Warn("failed to deliver cron notification",
			"channel", job.ChannelID, "job_id", job.ID, "error", err)
	}
}
