package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/util"
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

		// #11: A step must have exactly one of Tool or Prompt set, not both.
		if step.Tool != "" && step.Prompt != "" {
			return "", fmt.Errorf("step %d: cannot specify both tool %q and prompt; a step must use one or the other", i, step.Tool)
		}

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
	// #12: Built-in "script" tool fallback — executes script files from the
	// skill's scripts/ directory when no registered tool named "script" exists.
	// This allows directory-format skills to run shell scripts without requiring
	// an explicit script tool registration.
	if step.Tool == "script" {
		return e.executeScriptTool(ctx, step, skill, isFirst, input)
	}

	reg := e.agent.ToolRegistry()

	// Build tool arguments from step.Args, inject input on first step
	args := step.Args
	if args == nil {
		args = make(map[string]any)
	}
	if isFirst && input != "" {
		args["input"] = input
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

// executeScriptTool handles the built-in "script" tool which runs executable
// files from the skill's scripts/ directory. If the script file is not found
// or is not executable, a descriptive error is returned.
func (e *Executor) executeScriptTool(ctx context.Context, step Step, skill *Skill, isFirst bool, input string) (string, error) {
	args := step.Args
	if args == nil {
		args = make(map[string]any)
	}
	if isFirst && input != "" {
		args["input"] = input
	}

	// Resolve the script file path relative to the skill directory.
	fileName, _ := args["file"].(string)
	if fileName == "" {
		return "", fmt.Errorf("script step: missing required \"file\" argument")
	}

	var scriptPath string
	if skill.Dir != "" {
		scriptPath = filepath.Join(skill.Dir, "scripts", fileName)
	} else {
		scriptPath = fileName
	}
	scriptPath = filepath.Clean(scriptPath)
	if skill.Dir != "" {
		scriptsDir := filepath.Join(skill.Dir, "scripts")
		if !util.IsSubpath(scriptPath, scriptsDir) {
			return "", fmt.Errorf("script step: script path %q escapes scripts directory", fileName)
		}
	}

	// Verify the script file exists and is executable.
	info, err := os.Stat(scriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("script step: script file not found: %s", scriptPath)
		}
		return "", fmt.Errorf("script step: cannot access script file %s: %w", scriptPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("script step: %s is a directory, not a script file", scriptPath)
	}

	// Build command arguments: pass remaining args (excluding "file" and "input") as CLI args.
	// Input is passed via stdin only, not duplicated in CLI args.
	var cmdArgs []string
	for k, v := range args {
		if k == "file" || k == "input" {
			continue
		}
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%v", k, v))
	}

	cmd := exec.CommandContext(ctx, scriptPath, cmdArgs...)
	if isFirst && input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("script %s failed: %w\noutput:\n%s", scriptPath, err, string(out))
	}
	return string(out), nil
}
