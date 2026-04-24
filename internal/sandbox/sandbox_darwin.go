//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// sandboxExecLaunch runs a command in a macOS Seatbelt sandbox using sandbox-exec.
// sandbox-exec is an external launcher that forks the child and applies the Seatbelt
// policy, so no re-exec pattern is needed (unlike Linux Landlock).
//
// sandbox-exec is deprecated by Apple but remains the only viable CLI sandboxing
// mechanism on macOS. See GitHub issue #16 for tracking.
func sandboxExecLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req.Config == nil {
		return hostExec(ctx, req)
	}

	profile := buildSeatbeltProfile(req.Config)
	args := []string{"-p", profile, "--", req.Shell, req.ShellFlag, req.Command}

	cmd := exec.CommandContext(ctx, "sandbox-exec", args...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}

// buildSeatbeltProfile generates a Seatbelt profile string from a SandboxConfig.
// Strategy: allow everything by default, then deny file-write globally,
// then allow writes only to whitelisted paths. Optionally deny network.
//
// All paths are resolved via filepath.EvalSymlinks() because macOS /tmp is
// a symlink to /private/tmp and Seatbelt resolves symlinks internally.
func buildSeatbeltProfile(cfg *SandboxConfig) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n")

	// Writable: root path
	if cfg.Root != "" {
		if resolved, err := resolvePath(cfg.Root); err == nil {
			fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", resolved)
		}
	}

	// Writable: allow paths
	for _, p := range cfg.AllowPaths {
		if resolved, err := resolvePath(p); err == nil {
			fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", resolved)
		}
	}

	// Read-only paths
	for _, p := range cfg.ReadonlyPaths {
		if resolved, err := resolvePath(p); err == nil {
			fmt.Fprintf(&b, "(allow file-read* (subpath %q))\n", resolved)
		}
	}

	// Default writable: /private/tmp if nothing else
	if cfg.Root == "" && len(cfg.AllowPaths) == 0 {
		b.WriteString("(allow file-write* (subpath \"/private/tmp\"))\n")
	}

	// Network restriction
	if !cfg.AllowNetwork {
		b.WriteString("(deny network*)\n")
	}

	return b.String()
}

// resolvePath resolves symlinks and returns a clean absolute path.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Path may not exist yet; use the absolute path as-is.
		return filepath.Clean(abs), nil
	}
	return resolved, nil
}

// hostExec runs a command directly on the host (fallback).
func hostExec(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	cmd := exec.CommandContext(ctx, req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}
