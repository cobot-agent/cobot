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

func launchProcessWithSandbox(ctx context.Context, command string, args []string, dir string, cfg *SandboxConfig) (*exec.Cmd, error) {
	if cfg == nil || cfg.IsEmpty() {
		return launchProcessDirect(ctx, command, args, dir)
	}

	// Skip sandbox in test binaries — they don't call HandleSandboxChildMode.
	exe, err := os.Executable()
	if err == nil && (strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(exe, ".test")) {
		return launchProcessDirect(ctx, command, args, dir)
	}

	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return launchProcessDirect(ctx, command, args, dir)
	}

	profile := buildSeatbeltProfile(cfg)
	allArgs := append([]string{"-p", profile, "--", command}, args...)
	cmd := exec.CommandContext(ctx, sandboxExecPath, allArgs...)
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
