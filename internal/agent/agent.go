package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// --- Session ---

const maxMessages = 1000

type Session struct {
	mu       sync.RWMutex
	messages []cobot.Message
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Messages() []cobot.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cobot.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// MessagesSnapshot returns a copy of the current messages along with the
// length at the time of the snapshot. This allows callers to later merge
// any messages appended after the snapshot was taken.
func (s *Session) MessagesSnapshot() ([]cobot.Message, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.messages)
	out := make([]cobot.Message, n)
	copy(out, s.messages)
	return out, n
}

func (s *Session) AddMessage(m cobot.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
	if len(s.messages) > maxMessages {
		if len(s.messages) > 0 && s.messages[0].Role == cobot.RoleSystem {
			keep := s.messages[len(s.messages)-(maxMessages-1):]
			kept := make([]cobot.Message, 0, maxMessages)
			kept = append(kept, s.messages[0])
			kept = append(kept, keep...)
			s.messages = kept
		} else {
			s.messages = s.messages[len(s.messages)-maxMessages:]
		}
	}
}

// --- Agent ---

type Agent struct {
	config             *cobot.Config
	sessionConfig      cobot.SessionConfig
	provider           cobot.Provider
	registry           cobot.ModelResolver
	tools              cobot.ToolRegistry
	session            *Session
	sessionID          string
	sessionStore       *SessionStore
	memoryStore        cobot.MemoryStore
	memoryRecall       cobot.MemoryRecall
	usageTracker       *UsageTracker
	systemPrompt       string
	sysPromptMu        sync.RWMutex
	streamMu           sync.Mutex // serializes concurrent Stream calls
	streamWg           sync.WaitGroup
	agentCtx           context.Context
	agentCancel        context.CancelFunc
	turnCount          int
	compressor         *Compressor
	compressMu         sync.Mutex // prevents concurrent compression runs
	stmPromoteInterval int        // turns between STM promotions (0 = disabled)
	cronScheduler      CronScheduler
}

// CronScheduler is a minimal interface for stopping the cron scheduler.
// This avoids a circular dependency between agent and cron packages.
type CronScheduler interface {
	Stop()
}

func New(config *cobot.Config, toolRegistry cobot.ToolRegistry) *Agent {
	agentCtx, agentCancel := context.WithCancel(context.Background())
	return &Agent{
		config:             config,
		sessionConfig:      config.Session,
		tools:              toolRegistry,
		session:            NewSession(),
		sessionID:          uuid.New().String(),
		usageTracker:       NewUsageTracker(),
		agentCtx:           agentCtx,
		agentCancel:        agentCancel,
		stmPromoteInterval: 10,
	}
}

func (a *Agent) SetSystemPrompt(prompt string) error {
	a.sysPromptMu.Lock()
	defer a.sysPromptMu.Unlock()
	a.systemPrompt = prompt
	return nil
}

func (a *Agent) GetSystemPrompt() string {
	a.sysPromptMu.RLock()
	defer a.sysPromptMu.RUnlock()
	return a.systemPrompt
}

func (a *Agent) SetProvider(p cobot.Provider) {
	a.provider = p
}

func (a *Agent) SetRegistry(r cobot.ModelResolver) {
	a.registry = r
}

func (a *Agent) Registry() cobot.ModelResolver {
	return a.registry
}

func (a *Agent) ToolRegistry() cobot.ToolRegistry {
	return a.tools
}

func (a *Agent) Session() *Session {
	return a.session
}

func (a *Agent) AddMessage(m cobot.Message) {
	a.session.AddMessage(m)
	a.persistMessage(m)
}

func (a *Agent) SetSessionStore(s *SessionStore) {
	a.sessionStore = s
}

func (a *Agent) SetSessionConfig(sc cobot.SessionConfig) {
	a.sessionConfig = sc
}

func (a *Agent) SessionConfig() cobot.SessionConfig {
	return a.sessionConfig
}

func (a *Agent) SessionID() string {
	return a.sessionID
}

func (a *Agent) persistSession() {
	if a.sessionStore == nil {
		return
	}
	if err := a.sessionStore.Save(a.sessionID, a.session, a.usageTracker.Get(), a.config.Model); err != nil {
		slog.Debug("failed to persist session", "err", err)
	}
}

// persistMessage appends a single message to the session JSONL file.
func (a *Agent) persistMessage(m cobot.Message) {
	if a.sessionStore == nil {
		return
	}
	if err := a.sessionStore.InitSession(a.sessionID, a.config.Model); err != nil {
		slog.Debug("failed to init session", "err", err)
		return
	}
	if err := a.sessionStore.AppendMessage(a.sessionID, m); err != nil {
		slog.Debug("failed to persist message", "err", err)
	}
}

// PersistUsage appends a usage snapshot to the session JSONL file.
func (a *Agent) PersistUsage() {
	if a.sessionStore == nil {
		return
	}
	if err := a.sessionStore.AppendUsage(a.sessionID, a.usageTracker.Get()); err != nil {
		slog.Debug("failed to persist usage", "err", err)
	}
}

func (a *Agent) RegisterTool(tool cobot.Tool) {
	a.tools.Register(tool)
}

func (a *Agent) SetMemoryStore(s cobot.MemoryStore) {
	a.memoryStore = s
}

func (a *Agent) MemoryStore() cobot.MemoryStore {
	return a.memoryStore
}

func (a *Agent) SetMemoryRecall(r cobot.MemoryRecall) {
	a.memoryRecall = r
}

func (a *Agent) MemoryRecall() cobot.MemoryRecall {
	return a.memoryRecall
}

func (a *Agent) SetCronScheduler(s CronScheduler) {
	a.cronScheduler = s
}

func (a *Agent) Config() *cobot.Config {
	return a.config
}

func (a *Agent) Provider() cobot.Provider {
	return a.provider
}

func (a *Agent) SetModel(modelSpec string) error {
	if a.registry != nil {
		p, modelName, err := a.registry.ProviderForModel(modelSpec)
		if err != nil {
			return err
		}
		if v, ok := p.(cobot.ModelValidator); ok {
			if err := v.ValidateModel(a.agentCtx, modelName); err != nil {
				return err
			}
		}
		a.provider = p
		a.config.Model = modelName
		a.initCompressor()
		return nil
	}
	a.config.Model = modelSpec
	a.initCompressor()
	return nil
}

func (a *Agent) initCompressor() {
	if a.provider == nil {
		return
	}
	ctxWindow := ContextWindowForModel(a.config.Model, nil)
	a.compressor = NewCompressor(a.sessionConfig, ctxWindow, a.provider, a.config.Model)
}

func (a *Agent) Model() string {
	return a.config.Model
}

func (a *Agent) SessionUsage() cobot.Usage {
	return a.usageTracker.Get()
}

func (a *Agent) ResetUsage() {
	a.usageTracker.Reset()
}

// deriveCtx returns a context derived from agentCtx that also cancels if the
// supplied ctx cancels. This ensures that agent-level cancellation (via Close)
// propagates into all in-flight Prompt/Stream calls.
func (a *Agent) deriveCtx(ctx context.Context) context.Context {
	derived, derivedCancel := context.WithCancel(a.agentCtx)

	go func() {
		select {
		case <-ctx.Done():
			derivedCancel()
		case <-a.agentCtx.Done():
			derivedCancel()
		}
	}()

	return derived
}

func (a *Agent) Close() error {
	if a.agentCancel != nil {
		a.agentCancel()
	}

	done := make(chan struct{})
	go func() {
		a.streamWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		// Force proceed after timeout rather than blocking indefinitely.
	}

	// Stop cron scheduler if running.
	if a.cronScheduler != nil {
		a.cronScheduler.Stop()
	}

	// Promote valuable STM items to LTM before closing the memory store.
	if a.memoryStore != nil {
		if stm, ok := a.memoryStore.(cobot.ShortTermMemory); ok {
			go func() {
				_ = stm.PromoteToLongTerm(context.Background(), a.sessionID)
				_ = stm.ClearShortTerm(context.Background(), a.sessionID)
			}()
		}
		// Give background promotion a moment to finish.
		time.Sleep(100 * time.Millisecond)
		if err := a.memoryStore.Close(); err != nil {
			return fmt.Errorf("close memory store: %w", err)
		}
	}
	return nil
}
