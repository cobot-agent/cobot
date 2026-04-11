package agent

import (
	"fmt"

	"github.com/cobot-agent/cobot/internal/memory"
	"github.com/cobot-agent/cobot/internal/tools"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type Agent struct {
	config      *cobot.Config
	provider    cobot.Provider
	tools       *tools.Registry
	session     *Session
	memoryStore *memory.Store
}

func New(config *cobot.Config) *Agent {
	return &Agent{
		config:  config,
		tools:   tools.NewRegistry(),
		session: NewSession(),
	}
}

func (a *Agent) SetProvider(p cobot.Provider) {
	a.provider = p
}

func (a *Agent) ToolRegistry() *tools.Registry {
	return a.tools
}

func (a *Agent) Session() *Session {
	return a.session
}

func (a *Agent) RegisterTool(tool cobot.Tool) {
	a.tools.Register(tool)
}

func (a *Agent) SetToolRegistry(r *tools.Registry) {
	a.tools = r
}

func (a *Agent) SetMemoryStore(s *memory.Store) {
	a.memoryStore = s
	a.tools.Register(memory.NewMemorySearchTool(s))
	a.tools.Register(memory.NewMemoryStoreTool(s))
}

func (a *Agent) MemoryStore() *memory.Store {
	return a.memoryStore
}

func (a *Agent) Config() *cobot.Config {
	return a.config
}

func (a *Agent) Provider() cobot.Provider {
	return a.provider
}

func (a *Agent) Close() error {
	if a.memoryStore != nil {
		if err := a.memoryStore.Close(); err != nil {
			return fmt.Errorf("close memory store: %w", err)
		}
	}
	return nil
}
