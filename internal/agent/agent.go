package agent

import (
	"github.com/cobot-agent/cobot/internal/tools"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type Agent struct {
	config   *cobot.Config
	provider cobot.Provider
	tools    *tools.Registry
	session  *Session
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

func (a *Agent) Close() error {
	return nil
}
