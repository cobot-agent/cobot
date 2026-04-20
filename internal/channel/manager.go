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

type channelEntry struct {
	ch        cobot.Channel
	sessionID string
}

// Manager tracks active channels and routes messages to them.
type Manager struct {
	mu       sync.RWMutex
	channels map[string][]channelEntry // channelID -> entries
	lastHB   map[string]time.Time      // last heartbeat timestamp per sessionID
	local    map[string]struct{}       // local sessionIDs never expire
	cancelHC context.CancelFunc        // stops health check goroutine
	hcDone   chan struct{}             // signals health check goroutine exited
}

func NewManager() *Manager {
	return &Manager{
		channels: make(map[string][]channelEntry),
		lastHB:   make(map[string]time.Time),
		local:    make(map[string]struct{}),
	}
}

// Register adds a channel to the manager and records an initial heartbeat.
// If the sessionID is already registered, the heartbeat is updated without
// adding a duplicate entry.
func (m *Manager) Register(ch cobot.Channel, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check for duplicate sessionID — update heartbeat if already registered.
	entries := m.channels[ch.ID()]
	for _, e := range entries {
		if e.sessionID == sessionID {
			m.lastHB[sessionID] = time.Now()
			return
		}
	}
	m.channels[ch.ID()] = append(m.channels[ch.ID()], channelEntry{ch: ch, sessionID: sessionID})
	m.lastHB[sessionID] = time.Now()
}

// Unregister removes a channel entry from the manager.
func (m *Manager) Unregister(channelID, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.channels[channelID]
	for i, e := range entries {
		if e.sessionID == sessionID {
			// remove entry at i
			entries[i] = entries[len(entries)-1]
			entries = entries[:len(entries)-1]
			break
		}
	}
	if len(entries) == 0 {
		delete(m.channels, channelID)
	} else {
		m.channels[channelID] = entries
	}
	delete(m.lastHB, sessionID)
	delete(m.local, sessionID)
}

// Get returns a channel by ID and whether it exists and is alive.
func (m *Manager) Get(id string) (cobot.Channel, bool) {
	m.mu.RLock()
	src := m.channels[id]
	entries := make([]channelEntry, len(src))
	copy(entries, src)
	m.mu.RUnlock()
	for _, e := range entries {
		if e.ch.IsAlive() {
			return e.ch, true
		}
	}
	return nil, false
}

// AllAliveIDs returns the IDs of all alive channels.
func (m *Manager) AllAliveIDs() []string {
	m.mu.RLock()
	channelsCopy := make(map[string][]channelEntry, len(m.channels))
	for k, v := range m.channels {
		entries := make([]channelEntry, len(v))
		copy(entries, v)
		channelsCopy[k] = entries
	}
	m.mu.RUnlock()

	var ids []string
	for id, entries := range channelsCopy {
		for _, e := range entries {
			if e.ch.IsAlive() {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// Notify delivers a message to the specified channel (implements cobot.Notifier).
// It fans out to all registered instances for the channel.
func (m *Manager) Notify(ctx context.Context, channelID string, msg cobot.ChannelMessage) {
	m.mu.RLock()
	entries := make([]channelEntry, len(m.channels[channelID]))
	copy(entries, m.channels[channelID])
	m.mu.RUnlock()

	if len(entries) == 0 {
		slog.Debug("notify: channel not found", "channel", channelID)
		return
	}

	// Fan out to all alive instances concurrently.
	var wg sync.WaitGroup
	for _, e := range entries {
		if !e.ch.IsAlive() {
			slog.Debug("notify: skipping dead channel instance", "channel", channelID, "session", e.sessionID)
			continue
		}
		wg.Add(1)
		go func(entry channelEntry) {
			defer wg.Done()
			if err := entry.ch.Send(ctx, msg); err != nil {
				slog.Warn("failed to deliver notification",
					"channel", channelID, "session", entry.sessionID, "error", err)
			}
		}(e)
	}
	wg.Wait()
}

// Heartbeat records a heartbeat from the given session.
// Returns error if the session is not registered.
func (m *Manager) Heartbeat(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.lastHB[sessionID]; !ok {
		return fmt.Errorf("session %q not registered", sessionID)
	}
	m.lastHB[sessionID] = time.Now()
	return nil
}

// MarkLocal marks a session as local (in-process). Local sessions are never
// expired by the health check, since they share the process lifetime.
func (m *Manager) MarkLocal(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.local[sessionID] = struct{}{}
}

// StartHealthCheck begins periodic expiry of sessions whose last heartbeat
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

	timeout := interval * 3 // session is dead if no heartbeat for 3 intervals
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

// expireStale removes sessions whose last heartbeat exceeds the timeout.
// Local sessions are never expired.
func (m *Manager) expireStale(timeout time.Duration) {
	m.mu.Lock()
	now := time.Now()
	var toClose []cobot.Channel
	for channelID, entries := range m.channels {
		var kept []channelEntry
		for _, e := range entries {
			if _, isLocal := m.local[e.sessionID]; isLocal {
				kept = append(kept, e)
				continue
			}
			last, ok := m.lastHB[e.sessionID]
			if ok && now.Sub(last) <= timeout {
				kept = append(kept, e)
				continue
			}
			// expired
			delete(m.lastHB, e.sessionID)
			delete(m.local, e.sessionID)
			toClose = append(toClose, e.ch)
		}
		if len(kept) == 0 {
			delete(m.channels, channelID)
		} else {
			m.channels[channelID] = kept
		}
	}
	m.mu.Unlock()

	for _, ch := range toClose {
		slog.Warn("channel heartbeat timeout, removing", "channel", ch.ID())
		ch.Close()
	}
}
