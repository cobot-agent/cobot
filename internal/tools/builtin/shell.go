package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type shellExecArgs struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
}

const defaultShellTimeout = 2 * time.Minute

type ShellExecTool struct {
	workdir         string
	blockedCommands []string
	allowNetwork    bool
	timeout         time.Duration
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

func WithShellTimeout(d time.Duration) ShellExecToolOption {
	return func(t *ShellExecTool) { t.timeout = d }
}

var networkCommands = []string{
	"curl", "wget", "ssh", "scp", "sftp", "nc", "ncat", "netcat",
	"telnet", "ftp", "rsync", "ping", "nslookup", "dig", "host",
}

func NewShellExecTool(opts ...ShellExecToolOption) *ShellExecTool {
	t := &ShellExecTool{
		allowNetwork: false,
		timeout:      defaultShellTimeout,
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
	cmdStr := a.Command
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return "", fmt.Errorf("empty command")
	}
	baseCmd := filepath.Base(fields[0])

	for _, blocked := range t.blockedCommands {
		if baseCmd == blocked || strings.HasPrefix(cmdStr, blocked+" ") || cmdStr == blocked {
			return "", fmt.Errorf("command %q is blocked", blocked)
		}
		if strings.Contains(cmdStr, "|"+blocked) || strings.Contains(cmdStr, ">"+blocked) || strings.Contains(cmdStr, "<"+blocked) || strings.Contains(cmdStr, "; "+blocked) {
			return "", fmt.Errorf("command %q is blocked", blocked)
		}
		if strings.Contains(cmdStr, "$("+blocked) || strings.Contains(cmdStr, "`"+blocked+"`") {
			return "", fmt.Errorf("command %q is blocked", blocked)
		}
	}
	if !t.allowNetwork {
		for _, nc := range networkCommands {
			if baseCmd == nc || strings.HasPrefix(cmdStr, nc+" ") || cmdStr == nc ||
				strings.Contains(cmdStr, " "+nc+" ") || strings.Contains(cmdStr, "|"+nc+" ") ||
				strings.Contains(cmdStr, "| "+nc+" ") || strings.Contains(cmdStr, ">"+nc) ||
				strings.Contains(cmdStr, "<"+nc) || strings.Contains(cmdStr, "; "+nc) ||
				strings.Contains(cmdStr, "$("+nc) || strings.Contains(cmdStr, "`"+nc+"`") {
				return "", fmt.Errorf("network command %q is blocked (allow_network is false)", nc)
			}
		}
	}
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", a.Command)
	if a.Dir != "" {
		if t.workdir != "" {
			absDir, err := filepath.Abs(a.Dir)
			if err != nil {
				return "", fmt.Errorf("resolve dir: %w", err)
			}
			absWorkdir, err := filepath.Abs(t.workdir)
			if err != nil {
				return "", fmt.Errorf("resolve workdir: %w", err)
			}
			rel, err := filepath.Rel(absWorkdir, absDir)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("dir %q is outside workspace boundaries", a.Dir)
			}
			cmd.Dir = absDir
		} else {
			cmd.Dir = a.Dir
		}
	} else if t.workdir != "" {
		cmd.Dir = t.workdir
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("shell command timed out after %s", t.timeout)
	}
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

var _ cobot.Tool = (*ShellExecTool)(nil)
