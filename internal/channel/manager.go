package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// Manager tracks active channels and routes messages to them.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]cobot.Channel
	lastHB   map[string]time.Time // last heartbeat timestamp per channel
	local    map[string]struct{}  // local channels never expire
	cancelHC context.CancelFunc   // stops health check goroutine
	hcDone   chan struct{}        // signals health check goroutine exited
}

func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]cobot.Channel),
		lastHB:   make(map[string]time.Time),
		local:    make(map[string]struct{}),
	}
}

// Register adds a channel to the manager and records an initial heartbeat.
func (m *Manager) Register(ch cobot.Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.ID()] = ch
	m.lastHB[ch.ID()] = time.Now()
}

// Unregister removes a channel from the manager.
func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
	delete(m.lastHB, id)
	delete(m.local, id)
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
	channels := make([]cobot.Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.RUnlock()

	var ids []string
	for _, ch := range channels {
		if ch.IsAlive() {
			ids = append(ids, ch.ID())
		}
	}
	sort.Strings(ids)
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

// Heartbeat records a heartbeat from the given channel.
// Returns error if the channel is not registered.
// This is called by the channel (or its proxy) to signal it is still alive.
func (m *Manager) Heartbeat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[id]; !ok {
		return fmt.Errorf("channel %q not registered", id)
	}
	m.lastHB[id] = time.Now()
	return nil
}

// MarkLocal marks a channel as local (in-process). Local channels are never
// expired by the health check, since they share the process lifetime.
func (m *Manager) MarkLocal(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.local[id] = struct{}{}
}

// StartHealthCheck begins periodic expiry of channels whose last heartbeat
// exceeds 3× the given interval. Dead channels are closed and unregistered.
// Call StopHealthCheck to terminate. If a previous health check is running it
// is stopped first.
func (m *Manager) StartHealthCheck(parent context.Context, interval time.Duration) {
	m.StopHealthCheck()

	if interval <= 0 {
		slog.Warn("health check interval must be positive, skipping", "interval", interval)
		return
	}

	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})

	m.mu.Lock()
	m.cancelHC = cancel
	m.hcDone = done
	m.mu.Unlock()

	timeout := interval * 3 // channel is dead if no heartbeat for 3 intervals
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.expireStale(timeout)
			}
		}
	}()
}

// StopHealthCheck stops the background health check goroutine and waits for it to exit.
// It is safe to call StopHealthCheck multiple times.
func (m *Manager) StopHealthCheck() {
	m.mu.Lock()
	cancel := m.cancelHC
	done := m.hcDone
	m.cancelHC = nil
	m.hcDone = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
		<-done
	}
}

// expireStale removes channels whose last heartbeat exceeds the timeout.
// Local channels are never expired.
func (m *Manager) expireStale(timeout time.Duration) {
	m.mu.RLock()
	now := time.Now()
	var stale []string
	for id := range m.channels {
		if _, isLocal := m.local[id]; isLocal {
			continue
		}
		last, ok := m.lastHB[id]
		if !ok || now.Sub(last) > timeout {
			stale = append(stale, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range stale {
		m.mu.RLock()
		ch, ok := m.channels[id]
		last := m.lastHB[id] // capture while locked
		m.mu.RUnlock()
		if ok {
			slog.Warn("channel heartbeat timeout, removing",
				"channel", id, "last_heartbeat", last.Format(time.RFC3339))
			ch.Close()
			m.Unregister(id)
		}
	}
}
