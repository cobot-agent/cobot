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

func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, error) {
	if !shouldUseReexec() || cfg == nil || cfg.IsEmpty() {
		return launchProcessDirect(ctx, command, args, dir)
	}

	exe, err := os.Executable()
	if err != nil {
		return launchProcessDirect(ctx, command, args, dir)
	}

	reexecArgs := buildReexecArgs(command, args, cfg)

	cmd := exec.CommandContext(ctx, exe, reexecArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil
}

func launchProcessDirect(ctx context.Context, command string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil
}
