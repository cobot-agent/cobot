package builtin

import (
	"context"
	"encoding/json"
	"os/exec"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type shellExecArgs struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
}

type ShellExecTool struct{}

func NewShellExecTool() *ShellExecTool {
	return &ShellExecTool{}
}

func (t *ShellExecTool) Name() string {
	return "shell_exec"
}

func (t *ShellExecTool) Description() string {
	return "Execute a shell command and return its output"
}

func (t *ShellExecTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"},"dir":{"type":"string"}},"required":["command"]}`)
}

func (t *ShellExecTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a shellExecArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", a.Command)
	if a.Dir != "" {
		cmd.Dir = a.Dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

var _ cobot.Tool = (*ShellExecTool)(nil)
