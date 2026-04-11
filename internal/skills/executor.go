package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
)

type Executor struct {
	agent *agent.Agent
}

func NewExecutor(a *agent.Agent) *Executor {
	return &Executor{agent: a}
}

func (e *Executor) Execute(ctx context.Context, skill *Skill, input string) (string, error) {
	var results []string
	for i, step := range skill.Steps {
		prompt := step.Prompt
		if i == 0 && input != "" {
			prompt = prompt + "\n\n" + input
		}

		resp, err := e.agent.Prompt(ctx, prompt)
		if err != nil {
			return strings.Join(results, "\n---\n"), fmt.Errorf("step %d (%s): %w", i, step.Output, err)
		}

		if step.Output != "" {
			results = append(results, fmt.Sprintf("[%s]\n%s", step.Output, resp.Content))
		}
	}

	if len(results) == 0 {
		return "Skill completed with no output steps", nil
	}
	return strings.Join(results, "\n---\n"), nil
}
