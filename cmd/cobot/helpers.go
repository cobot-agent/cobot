package main

import (
	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/bootstrap"
	"github.com/cobot-agent/cobot/internal/workspace"
	cobot "github.com/cobot-agent/cobot/pkg"
)

// initAgent delegates to bootstrap.InitAgent and unpacks the result into
// the (agent, workspace, cleanup, error) tuple that existing callers expect.
func initAgent(cfg *cobot.Config, requireProvider bool) (*agent.Agent, *workspace.Workspace, func(), error) {
	res, err := bootstrap.InitAgent(cfg, requireProvider)
	if err != nil {
		return nil, nil, nil, err
	}
	return res.Agent, res.Workspace, res.Cleanup, nil
}

// reconfigureAgentForWorkspace delegates to bootstrap.ConfigureAgentForWorkspace.
func reconfigureAgentForWorkspace(a *agent.Agent, ws *workspace.Workspace, registry cobot.ModelResolver) error {
	return bootstrap.ConfigureAgentForWorkspace(a, ws, registry)
}
