package cobot

import "context"

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

func (a *Agent) SetProvider(p Provider) {
	a.provider = p
	a.core.SetProvider(p)
}

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
