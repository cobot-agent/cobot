package command

import (
	"context"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/pkg"
)

type memoryCmd struct{}

func (c *memoryCmd) Name() string   { return "memory" }
func (c *memoryCmd) Help() string   { return "manage session memory (/memory clear)" }
func (c *memoryCmd) Execute(ctx context.Context, cmdCtx cobot.CommandContext) (*cobot.OutboundMessage, error) {
	args := cmdCtx.Text

	switch {
	case args == "" || args == "clear":
		return &cobot.OutboundMessage{Text: "Memory clear not yet implemented."}, nil
	default:
		return &cobot.OutboundMessage{Text: "Usage: /memory clear"}, nil
	}
}

func (c *memoryCmd) clear(ctx context.Context, cmdCtx cobot.CommandContext) (*cobot.OutboundMessage, error) {
	a, ok := cmdCtx.Agent.(*agent.Agent)
	if !ok || a == nil {
		return &cobot.OutboundMessage{Text: "Agent not available."}, nil
	}
	_ = a.SessionMgr()
	return &cobot.OutboundMessage{Text: "Memory clear not yet implemented."}, nil
}
