//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
)

// landlockLaunch runs a command in a Landlock-sandboxed subprocess using the
// re-exec pattern: the current binary is re-invoked with a special child-mode
// argument, the child applies Landlock restrictions to itself, then execs the
// actual shell command.
func landlockLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return hostExec(ctx, req)
	}

	// Don't use re-exec in test binaries — they don't call HandleSandboxChildMode.
	if strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(exe, ".test") {
		return hostExec(ctx, req)
	}

	args := []string{"__cobot_sandbox_child__"}
	if req.Config != nil {
		if req.Config.Root != "" {
			args = append(args, "--root", req.Config.Root)
		}
		for _, p := range req.Config.AllowPaths {
			args = append(args, "--allow", p)
		}
		for _, p := range req.Config.ReadonlyPaths {
			args = append(args, "--readonly", p)
		}
		if !req.Config.AllowNetwork {
			args = append(args, "--no-network")
		}
	}
	args = append(args, "--")
	args = append(args, req.Shell, req.ShellFlag, req.Command)

	cmd := exec.CommandContext(ctx, exe, args...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}

// hostExec runs a command directly on the host (fallback when Landlock re-exec
// is unavailable).
func hostExec(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	cmd := exec.CommandContext(ctx, req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}

// HandleSandboxChildMode checks if this process was invoked as a sandbox child
// (via re-exec from landlockLaunch). If so, it applies Landlock restrictions
// and execs the target command. Returns true if child mode was handled (caller
// should exit), false for normal startup.
//
// This should be called early in main() before any other setup.
func HandleSandboxChildMode() bool {
	if len(os.Args) < 2 || os.Args[1] != "__cobot_sandbox_child__" {
		return false
	}

	args := os.Args[2:]
	var (
		root       string
		allowPaths []string
		roPaths    []string
		noNetwork  bool
		cmdArgs    []string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			i++
			if i < len(args) {
				root = args[i]
			}
		case "--allow":
			i++
			if i < len(args) {
				allowPaths = append(allowPaths, args[i])
			}
		case "--readonly":
			i++
			if i < len(args) {
				roPaths = append(roPaths, args[i])
			}
		case "--no-network":
			noNetwork = true
		case "--":
			cmdArgs = args[i+1:]
		}
	}

	if len(cmdArgs) < 3 {
		fmt.Fprintln(os.Stderr, "cobot-sandbox: missing command")
		os.Exit(1)
	}

	applyLandlock(root, allowPaths, roPaths, noNetwork)

	// Exec the shell command.
	shell := cmdArgs[0]
	shellArgs := cmdArgs[1:]

	cmd := exec.Command(shell, shellArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "cobot-sandbox: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
	return true // unreachable
}

// applyLandlock applies filesystem and network restrictions using the Linux
// Landlock LSM. BestEffort mode gracefully degrades on kernels without Landlock
// support (older kernels, custom builds without CONFIG_LANDLOCK).
func applyLandlock(root string, allowPaths, roPaths []string, noNetwork bool) {
	cfg := landlock.V4.BestEffort()

	var rules []landlock.Rule

	// System paths are always readable (binaries, libraries, configs).
	rules = append(rules, landlock.RODirs("/usr", "/bin", "/sbin", "/lib", "/lib64", "/etc"))

	// Writable paths from sandbox config.
	if root != "" {
		rules = append(rules, landlock.RWDirs(root))
	}
	for _, p := range allowPaths {
		rules = append(rules, landlock.RWDirs(p))
	}

	// Explicitly read-only paths.
	for _, p := range roPaths {
		rules = append(rules, landlock.RODirs(p))
	}

	// Default: allow writing to /tmp if nothing else is writable.
	if root == "" && len(allowPaths) == 0 {
		rules = append(rules, landlock.RWDirs("/tmp"))
	}

	if err := cfg.RestrictPaths(rules...); err != nil {
		// BestEffort should never hard-fail, but log if it does.
		fmt.Fprintf(os.Stderr, "cobot-sandbox: landlock: %v\n", err)
	}

	// Network restriction: deny all TCP bind/connect.
	if noNetwork {
		if err := cfg.RestrictNet(); err != nil {
			fmt.Fprintf(os.Stderr, "cobot-sandbox: landlock net: %v\n", err)
		}
	}
}
