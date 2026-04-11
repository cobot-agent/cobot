package subagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/tools"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type Coordinator struct {
	parent    *agent.Agent
	mu        sync.RWMutex
	subagents map[string]*SubAgent
}

func NewCoordinator(parent *agent.Agent) *Coordinator {
	return &Coordinator{
		parent:    parent,
		subagents: make(map[string]*SubAgent),
	}
}

func (c *Coordinator) Spawn(ctx context.Context, config *Config) (*SubAgent, error) {
	id := "sub_" + newHexID(8)
	sa := &SubAgent{
		ID:     id,
		config: config,
		done:   make(chan struct{}),
	}

	c.mu.Lock()
	c.subagents[id] = sa
	c.mu.Unlock()

	go c.run(ctx, sa)
	return sa, nil
}

func (c *Coordinator) Get(id string) (*SubAgent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sa, ok := c.subagents[id]
	return sa, ok
}

func (c *Coordinator) Gather(ctx context.Context, ids []string) []*Result {
	var results []*Result
	for _, id := range ids {
		if sa, ok := c.Get(id); ok {
			select {
			case <-sa.Done():
				results = append(results, sa.Result())
			case <-ctx.Done():
				results = append(results, &Result{ID: id, Error: "timeout"})
			}
		}
	}
	return results
}

func (c *Coordinator) CancelAll() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, sa := range c.subagents {
		select {
		case <-sa.Done():
		default:
			sa.close()
		}
	}
}

func (c *Coordinator) run(ctx context.Context, sa *SubAgent) {
	defer sa.close()

	start := time.Now()
	timeout := sa.config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	maxTurns := sa.config.MaxTurns
	if maxTurns == 0 {
		maxTurns = 5
	}

	model := c.parent.Config().Model
	if sa.config.Model != "" {
		model = sa.config.Model
	}

	cfg := &cobot.Config{Model: model, MaxTurns: maxTurns}
	childAgent := agent.New(cfg)

	reg := tools.NewRegistry()
	if len(sa.config.ToolNames) == 0 {
		for _, def := range c.parent.ToolRegistry().ToolDefs() {
			if t, err := c.parent.ToolRegistry().Get(def.Name); err == nil {
				reg.Register(t)
			}
		}
	} else {
		for _, name := range sa.config.ToolNames {
			if t, err := c.parent.ToolRegistry().Get(name); err == nil {
				reg.Register(t)
			}
		}
	}
	childAgent.SetToolRegistry(reg)

	if c.parent.Provider() != nil {
		childAgent.SetProvider(c.parent.Provider())
	}

	resp, err := childAgent.Prompt(subCtx, sa.config.Task)
	duration := time.Since(start)

	sa.result = &Result{
		ID:       sa.ID,
		Duration: duration,
	}

	if err != nil {
		sa.result.Error = err.Error()
		return
	}

	if resp != nil {
		sa.result.Output = resp.Content
		sa.result.ToolCalls = len(resp.ToolCalls)
	}
}

func newHexID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
