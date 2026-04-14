package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cobot-agent/cobot/internal/util"
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
		// Check command substitution and pipe/redirection variants with
		// both tight (;rm) and spaced (; rm) forms to prevent bypasses.
		for _, sep := range []string{"|", ">", "<", ";", "&"} {
			if strings.Contains(cmdStr, sep+blocked+" ") || strings.Contains(cmdStr, sep+blocked) && !strings.ContainsAny(string(cmdStr[strings.Index(cmdStr, sep)+1]), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				// Tight form like ;rm or spaced form ; rm
			}
			if strings.Contains(cmdStr, sep+" "+blocked) || strings.Contains(cmdStr, sep+blocked) {
				return "", fmt.Errorf("command %q is blocked", blocked)
			}
		}
		// $(cmd) and `cmd` substitution — match backtick prefix broadly
		if strings.Contains(cmdStr, "$("+blocked) || strings.Contains(cmdStr, "`"+blocked) {
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
	shell, shellFlag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, shellFlag = "cmd", "/C"
	}
	cmd := exec.CommandContext(ctx, shell, shellFlag, a.Command)
	if a.Dir != "" {
		if t.workdir != "" {
			absWorkdir, err := filepath.Abs(t.workdir)
			if err != nil {
				return "", fmt.Errorf("resolve workdir: %w", err)
			}
			absDir := absWorkdir
			if filepath.IsAbs(a.Dir) {
				absDir = a.Dir
			} else {
				absDir = filepath.Join(absWorkdir, a.Dir)
				if absDir, err = filepath.Abs(absDir); err != nil {
					return "", fmt.Errorf("resolve dir: %w", err)
				}
			}
			absDir = util.EvalSymlinks(absDir)
			absWorkdir = util.EvalSymlinks(absWorkdir)
			if !util.IsSubpath(absDir, absWorkdir) {
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
