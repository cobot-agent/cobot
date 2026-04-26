//go:build !linux && !darwin && !windows

package sandbox

import (
	"context"
	"os/exec"
)

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return hostExec(ctx, req)
}

func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, func(), error) {
	cmd, err := launchProcessDirect(ctx, command, args, dir)
	return cmd, nil, err
}

func launchProcessDirect(ctx context.Context, command string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil
}
