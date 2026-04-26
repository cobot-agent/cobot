//go:build windows

package sandbox

import (
	"context"
	"os/exec"
	"syscall"
)

// platformLaunch uses restrictedTokenLaunch which wraps commands via shell.
// For LaunchProcess we need direct process creation, which requires
// CreateProcessAsUserW — see launchProcessWithSandbox below.

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return restrictedTokenLaunch(ctx, req)
}

// launchProcessWithSandbox launches a process with Windows Restricted Token isolation.
// It creates a restricted token with a capability SID, grants ACLs on allowed
// directories, and launches the process with that token. The process can only
// write to paths where the capability SID has been granted access.
func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, error) {
	if cfg == nil || cfg.IsEmpty() {
		return launchProcessDirect(ctx, command, args, dir)
	}

	// Windows Restricted Token setup is expensive (SID generation + ACL manipulation).
	// For short-lived commands, this overhead is significant. However, ACPSubAgent
	// uses this for long-running server processes, so the setup cost is amortized.
	return launchProcessWithRestrictedToken(ctx, command, args, dir, cfg)
}

func launchProcessDirect(ctx context.Context, command string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: 0,
	}
	return cmd, nil
}
