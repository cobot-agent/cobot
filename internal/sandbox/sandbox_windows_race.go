//go:build windows && race

package sandbox

import (
	"context"
	"os/exec"
)

// restrictedTokenLaunch is a no-op fallback when built with -race.
// CreateRestrictedToken + race detector causes STATUS_HEAP_CORRUPTION
// (0xc0000374) on Windows, so we fall back to unsandboxed execution.
func restrictedTokenLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return hostExec(ctx, req)
}

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return restrictedTokenLaunch(ctx, req)
}

// launchProcessWithSandbox falls back to direct process launch when built with -race.
func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, error) {
	return launchProcessDirect(ctx, command, args, dir)
}

func launchProcessDirect(ctx context.Context, command string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd, nil
}
