//go:build darwin

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return sandboxExecLaunch(ctx, req)
}

func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, func(), error) {
	if cfg == nil || cfg.IsEmpty() {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}

	// Skip sandbox in test binaries.
	exe, err := os.Executable()
	if err == nil && (strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(exe, ".test")) {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}

	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		cmd, err := launchProcessDirect(ctx, command, args, dir)
		return cmd, nil, err
	}

	profile := buildSeatbeltProfile(cfg)
	allArgs := append([]string{"-p", profile, "--", command}, args...)
	cmd := exec.CommandContext(ctx, sandboxExecPath, allArgs...)
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
