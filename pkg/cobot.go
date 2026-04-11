package cobot

import "context"

type Agent struct {
	config   *Config
	provider Provider
}

func New(config *Config) (*Agent, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &Agent{
		config: config,
	}, nil
}

func (a *Agent) Prompt(ctx context.Context, message string) (*ProviderResponse, error) {
	return nil, nil
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan Event, error) {
	return nil, nil
}

func (a *Agent) RegisterTool(tool Tool) error {
	return nil
}

func (a *Agent) Close() error {
	return nil
}
