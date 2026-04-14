package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
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
		var result string
		var err error

		if step.Tool != "" {
			// Tool-based step: execute a tool directly
			result, err = e.executeTool(ctx, step, skill, i == 0, input)
		} else if step.Prompt != "" {
			// Prompt-based step: ask the agent
			prompt := step.Prompt
			if i == 0 && input != "" {
				prompt = prompt + "\n\n" + input
			}
			var resp *cobot.ProviderResponse
			resp, err = e.agent.Prompt(ctx, prompt)
			if err == nil && resp != nil {
				result = resp.Content
			}
		} else {
			continue // skip empty steps
		}

		if err != nil {
			label := step.Output
			if label == "" {
				label = fmt.Sprintf("step %d", i)
			}
			return strings.Join(results, "\n---\n"), fmt.Errorf("step %d (%s): %w", i, label, err)
		}

		if step.Output != "" {
			results = append(results, fmt.Sprintf("[%s]\n%s", step.Output, result))
		}
	}

	if len(results) == 0 {
		return "Skill completed with no output steps", nil
	}
	return strings.Join(results, "\n---\n"), nil
}

func (e *Executor) executeTool(ctx context.Context, step Step, skill *Skill, isFirst bool, input string) (string, error) {
	reg := e.agent.ToolRegistry()

	// Build tool arguments from step.Args, inject input on first step
	args := step.Args
	if args == nil {
		args = make(map[string]any)
	}
	if isFirst && input != "" {
		args["input"] = input
	}

	// For script tools, resolve the file path relative to the skill directory
	if step.Tool == "script" && skill.Dir != "" {
		if file, ok := args["file"]; ok {
			args["file"] = fmt.Sprintf("%s/scripts/%s", skill.Dir, file)
		}
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal tool args: %w", err)
	}

	tool, err := reg.Get(step.Tool)
	if err != nil {
		return "", fmt.Errorf("tool %q not found: %w", step.Tool, err)
	}

	return tool.Execute(ctx, argsJSON)
}
