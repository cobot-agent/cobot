package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	config       *cobot.Config
	provider     cobot.Provider
	registry     cobot.ModelResolver
	tools        cobot.ToolRegistry
	session      *Session
	memoryStore  cobot.MemoryStore
	memoryRecall cobot.MemoryRecall
	usageTracker *UsageTracker
	systemPrompt string
	sysPromptMu  sync.RWMutex
	streamMu     sync.Mutex // serializes concurrent Stream calls
	streamWg     sync.WaitGroup
	agentCtx     context.Context
	agentCancel  context.CancelFunc
}

func New(config *cobot.Config, toolRegistry cobot.ToolRegistry) *Agent {
	agentCtx, agentCancel := context.WithCancel(context.Background())
	return &Agent{
		config:       config,
		tools:        toolRegistry,
		session:      NewSession(),
		usageTracker: NewUsageTracker(),
		agentCtx:     agentCtx,
		agentCancel:  agentCancel,
	}
}

func (a *Agent) SetSystemPrompt(prompt string) {
	a.sysPromptMu.Lock()
	defer a.sysPromptMu.Unlock()
	a.systemPrompt = prompt
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
}

func (a *Agent) RegisterTool(tool cobot.Tool) {
	a.tools.Register(tool)
}

func (a *Agent) SetToolRegistry(r cobot.ToolRegistry) {
	a.tools = r
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
		a.provider = p
		a.config.Model = modelName
		return nil
	}
	a.config.Model = modelSpec
	return nil
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

	if ctx.Err() != nil {
		derivedCancel()
	} else {
		context.AfterFunc(ctx, derivedCancel)
	}

	if a.agentCtx.Err() != nil {
		derivedCancel()
	} else {
		context.AfterFunc(a.agentCtx, derivedCancel)
	}

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

	if a.memoryStore != nil {
		if err := a.memoryStore.Close(); err != nil {
			return fmt.Errorf("close memory store: %w", err)
		}
	}
	return nil
}

// --- Context / prompt helpers ---

func (a *Agent) buildMessages(ctx context.Context) []cobot.Message {
	msgs := a.session.Messages()
	system := a.getSystemPrompt(ctx)
	if system == "" {
		return msgs
	}
	return append([]cobot.Message{{Role: cobot.RoleSystem, Content: system}}, msgs...)
}

func (a *Agent) getSystemPrompt(ctx context.Context) string {
	a.sysPromptMu.RLock()
	cached := a.systemPrompt
	a.sysPromptMu.RUnlock()

	if cached != "" {
		return cached
	}

	if a.memoryRecall == nil {
		return cobot.DefaultSystemPrompt
	}

	// Double-check locking: acquire write lock and re-check to avoid
	// redundant WakeUp calls from concurrent cache misses.
	a.sysPromptMu.Lock()
	if a.systemPrompt != "" {
		a.sysPromptMu.Unlock()
		return a.systemPrompt
	}

	wakeUp, err := a.memoryRecall.WakeUp(ctx)
	if err != nil || wakeUp == "" {
		a.sysPromptMu.Unlock()
		return cobot.DefaultSystemPrompt
	}

	a.systemPrompt = wakeUp
	a.sysPromptMu.Unlock()

	return wakeUp
}
