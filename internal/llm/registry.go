package llm

import (
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type Registry struct {
	providers map[string]cobot.Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]cobot.Provider),
	}
}

func (r *Registry) Register(name string, p cobot.Provider) {
	r.providers[name] = p
}

func (r *Registry) Get(name string) (cobot.Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
