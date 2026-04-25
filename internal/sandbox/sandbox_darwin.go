//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// sandboxExecLaunch runs a command in a macOS Seatbelt sandbox using sandbox-exec.
// sandbox-exec is deprecated by Apple but remains the only viable CLI sandboxing
// mechanism on macOS. See GitHub issue #16 for tracking.
func sandboxExecLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req.Config == nil {
		slog.Warn("sandbox: no config provided, running command without Seatbelt enforcement", "command", req.Command)
		return hostExec(ctx, req)
	}

	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		slog.Warn("sandbox: sandbox-exec not found, falling back to unsandboxed execution", "error", err)
		return hostExec(ctx, req)
	}

	profile := buildSeatbeltProfile(req.Config)
	cmd := exec.CommandContext(ctx, sandboxExecPath, "-p", profile, "--", req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}

// buildSeatbeltProfile generates a Seatbelt profile from a SandboxConfig.
// Strategy: (allow default), then selectively deny file-write and network,
// then allow writes to whitelisted paths.
// Paths are resolved via normalizePath because macOS /tmp → /private/tmp.
func buildSeatbeltProfile(cfg *SandboxConfig) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n")

	writePaths := make([]string, 0, 1+len(cfg.AllowPaths))
	if cfg.Root != "" {
		writePaths = append(writePaths, cfg.Root)
	}
	writePaths = append(writePaths, cfg.AllowPaths...)

	// No write paths configured: all writes remain denied by the blanket deny above.
	for _, p := range writePaths {
		if resolved, err := normalizePath(p); err == nil {
			fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", resolved)
		}
	}

	for _, p := range cfg.ReadonlyPaths {
		if resolved, err := normalizePath(p); err == nil {
			fmt.Fprintf(&b, "(allow file-read* (subpath %q))\n", resolved)
		}
	}

	// Explicitly deny writes to readonly paths, taking precedence over any
	// broader write-allow rules (e.g. Root subpath) due to Seatbelt's
	// last-matching-wins semantics.
	for _, p := range cfg.ReadonlyPaths {
		if resolved, err := normalizePath(p); err == nil {
			fmt.Fprintf(&b, "(deny file-write* (subpath %q))\n", resolved)
		}
	}

	if !cfg.AllowNetwork {
		b.WriteString("(deny network*)\n")
	}

	return b.String()
}
