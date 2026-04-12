package cobot

import (
	"context"
	"fmt"

	acp "github.com/cobot-agent/cobot/acp"
)

type AgentCore interface {
	SetProvider(Provider)
	Prompt(ctx context.Context, message string) (*ProviderResponse, error)
	Stream(ctx context.Context, message string) (<-chan Event, error)
	RegisterTool(Tool)
	Close() error
}

type Agent struct {
	core     AgentCore
	config   *Config
	provider Provider
	memStore MemoryStore
}

// CoreFactory is a function type that creates an AgentCore from a Config.
// This is used by NewFromConfig to create a default core without causing import cycles.
type CoreFactory func(*Config) (AgentCore, error)

// defaultCoreFactory is set by the internal/agent package during initialization.
var defaultCoreFactory CoreFactory

// RegisterCoreFactory allows the internal/agent package to register its core creation function.
// This avoids import cycles between pkg and internal packages.
func RegisterCoreFactory(factory CoreFactory) {
	defaultCoreFactory = factory
}

func New(config *Config, core AgentCore) (*Agent, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &Agent{
		core:   core,
		config: config,
	}, nil
}

// NewFromConfig creates an Agent using a default core internally created from the provided config.
// This preserves backward compatibility for New(config, core) while offering a simplified API.
// The default core factory must be registered by calling RegisterCoreFactory before using this function.
func NewFromConfig(config Config) (*Agent, error) {
	if defaultCoreFactory == nil {
		return nil, fmt.Errorf("no core factory registered: call RegisterCoreFactory first")
	}
	core, err := defaultCoreFactory(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to create default core: %w", err)
	}
	return New(&config, core)
}

func (a *Agent) SetProvider(p Provider) {
	a.provider = p
	a.core.SetProvider(p)
}

func (a *Agent) Provider() Provider { return a.provider }

func (a *Agent) SetMemoryStore(s MemoryStore) { a.memStore = s }

func (a *Agent) MemoryStore() MemoryStore { return a.memStore }

func (a *Agent) Config() *Config { return a.config }

func (a *Agent) Prompt(ctx context.Context, message string) (*ProviderResponse, error) {
	return a.core.Prompt(ctx, message)
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan Event, error) {
	return a.core.Stream(ctx, message)
}

func (a *Agent) RegisterTool(tool Tool) error {
	a.core.RegisterTool(tool)
	return nil
}

func (a *Agent) Close() error {
	return a.core.Close()
}

// ServeACP starts an ACP server on the provided address and blocks until the
// context is canceled or the server stops.
// It delegates to the internal ACP scaffolding to obtain or create a server
// and then starts serving on the given address.
func (a *Agent) ServeACP(ctx context.Context, addr string) error {
	// Use the public acp scaffold. Pass the public Agent as the scaffold's input
	// so the server has a representative reference.
	server := acp.NewServer(a)
	if server == nil {
		return fmt.Errorf("ACP server scaffold not available")
	}
	return server.Serve(ctx, addr)
}
