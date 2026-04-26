//go:build windows && !race

package sandbox

import (
	"context"
	"os/exec"
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
func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, func(), error) {
	if cfg == nil || cfg.IsEmpty() {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}
	return launchProcessWithRestrictedToken(ctx, command, args, dir, cfg)
}

func launchProcessDirect(ctx context.Context, command string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil
}
