package command

import (
	"context"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/cron"
	"github.com/cobot-agent/cobot/pkg"
)

// Registry is a helper that builds a registry, wires dependencies into command context,
// and provides the cobot.CommandRegistry interface.
type Registry struct {
	reg   cobot.CommandRegistry
	agent *agent.Agent
	cron  *cron.Scheduler
	cmds  []cobot.Command
}

// New creates a new command registry wired with the given agent and cron scheduler.
func New(a *agent.Agent, c *cron.Scheduler) *Registry {
	r := &Registry{
		reg:   cobot.NewCommandRegistry(),
		agent: a,
		cron:  c,
	}
	r.cmds = []cobot.Command{
		&exitCmd{},
		&stopCmd{},
		&helpCmd{},
		&modelCmd{},
		&skillCmd{},
		&memoryCmd{},
		&cronCmd{},
	}
	for _, cmd := range r.cmds {
		r.reg.Register(cmd)
	}
	return r
}

// SetHelpData populates help's Data with the command list.
func (r *Registry) SetHelpData() {
	for _, cmd := range r.cmds {
		if h, ok := cmd.(*helpCmd); ok {
			h.cmds = r.cmds
		}
	}
}

// SetCron sets the cron scheduler after registry creation.
func (r *Registry) SetCron(c *cron.Scheduler) { r.cron = c }

// Execute calls the inner registry's Execute and injects Agent and Data into cmdCtx.
func (r *Registry) Execute(ctx context.Context, cmdCtx cobot.CommandContext) (bool, error) {
	cmdCtx.Agent = r.agent
	cmdCtx.Data = r.cron
	return r.reg.Execute(ctx, cmdCtx)
}

// Registry returns the underlying CommandRegistry.
func (r *Registry) Registry() cobot.CommandRegistry { return r.reg }

// Commands implements cobot.CommandRegistry by delegating to the inner registry.
func (r *Registry) Commands() []cobot.Command { return r.reg.Commands() }

// Register implements cobot.CommandRegistry by delegating to the inner registry.
func (r *Registry) Register(cmd cobot.Command) { r.reg.Register(cmd) }
