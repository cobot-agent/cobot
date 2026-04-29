package command

import (
	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/cron"
)

// Context holds the dependencies available to all commands.
type Context struct {
	Agent *agent.Agent
	Cron  *cron.Scheduler
}
