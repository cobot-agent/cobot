package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type shellExecArgs struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
}

type ShellExecTool struct {
	workdir         string
	blockedCommands []string
}

type ShellExecToolOption func(*ShellExecTool)

func WithShellSandbox(workdir string, blocked []string) ShellExecToolOption {
	return func(t *ShellExecTool) { t.workdir = workdir; t.blockedCommands = blocked }
}

func NewShellExecTool(opts ...ShellExecToolOption) *ShellExecTool {
	t := &ShellExecTool{}
	for _, o := range opts {
		o(t)
	}
	return t
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
	for _, blocked := range t.blockedCommands {
		if strings.Contains(a.Command, blocked) {
			return "", fmt.Errorf("sandbox: blocked command: %s", a.Command)
		}
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", a.Command)
	if t.workdir != "" {
		cmd.Dir = t.workdir
	} else if a.Dir != "" {
		cmd.Dir = a.Dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

var _ cobot.Tool = (*ShellExecTool)(nil)
