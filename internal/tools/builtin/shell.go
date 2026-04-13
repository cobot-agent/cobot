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
	allowNetwork    bool
}

type ShellExecToolOption func(*ShellExecTool)

func WithShellWorkdir(workdir string) ShellExecToolOption {
	return func(t *ShellExecTool) { t.workdir = workdir }
}

func WithShellBlockedCommands(blocked []string) ShellExecToolOption {
	return func(t *ShellExecTool) { t.blockedCommands = blocked }
}

func WithShellAllowNetwork(allow bool) ShellExecToolOption {
	return func(t *ShellExecTool) { t.allowNetwork = allow }
}

var networkCommands = []string{
	"curl", "wget", "ssh", "scp", "sftp", "nc", "ncat", "netcat",
	"telnet", "ftp", "rsync", "ping", "nslookup", "dig", "host",
}

func NewShellExecTool(opts ...ShellExecToolOption) *ShellExecTool {
	t := &ShellExecTool{
		allowNetwork: true,
	}
	for _, opt := range opts {
		opt(t)
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
		if strings.HasPrefix(a.Command, blocked+" ") || a.Command == blocked {
			return "", fmt.Errorf("command %q is blocked", blocked)
		}
	}
	if !t.allowNetwork {
		for _, nc := range networkCommands {
			if strings.HasPrefix(a.Command, nc+" ") || a.Command == nc ||
				strings.Contains(a.Command, " "+nc+" ") || strings.Contains(a.Command, "|"+nc+" ") ||
				strings.Contains(a.Command, "| "+nc+" ") {
				return "", fmt.Errorf("network command %q is blocked (allow_network is false)", nc)
			}
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
