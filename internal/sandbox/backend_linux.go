//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
)

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return landlockLaunch(ctx, req)
}

func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, func(), error) {
	if cfg == nil || cfg.IsEmpty() {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}
	if !shouldUseReexec() {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}

	reexecArgs := buildReexecArgs(command, args, cfg)
	cmd := exec.CommandContext(ctx, exe, reexecArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil, nil
}

func launchProcessDirect(ctx context.Context, command string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil
}
