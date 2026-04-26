//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
	"golang.org/x/sys/unix"
)

// shouldUseReexec returns true if the re-exec pattern should be used for sandboxing.
// Returns false if running in a test binary (which doesn't call HandleSandboxChildMode)
// or if the executable cannot be determined.
func shouldUseReexec() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return !strings.HasSuffix(os.Args[0], ".test") && !strings.HasSuffix(exe, ".test")
}

// buildReexecArgs builds the argument slice for re-invoking the current binary
// in sandbox child mode, using the given command and arguments. The command is
// appended after the "--" separator.
func buildReexecArgs(command string, args []string, cfg *SandboxConfig) []string {
	reexecArgs := []string{"__cobot_sandbox_child__"}
	if cfg.Root != "" {
		reexecArgs = append(reexecArgs, "--root", cfg.Root)
	}
	for _, p := range cfg.AllowPaths {
		reexecArgs = append(reexecArgs, "--allow", p)
	}
	for _, p := range cfg.ReadonlyPaths {
		reexecArgs = append(reexecArgs, "--readonly", p)
	}
	if !cfg.AllowNetwork {
		reexecArgs = append(reexecArgs, "--no-network")
	}
	reexecArgs = append(reexecArgs, "--")
	reexecArgs = append(reexecArgs, command)
	reexecArgs = append(reexecArgs, args...)
	return reexecArgs
}

// landlockLaunch runs a command in a Landlock-sandboxed subprocess using the
// re-exec pattern: the current binary is re-invoked with a special child-mode
// argument, the child applies Landlock restrictions to itself, then execs the
// actual shell command.
func landlockLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if !shouldUseReexec() {
		return hostExec(ctx, req)
	}

	if req.Config == nil || req.Config.IsEmpty() {
		// No config means no sandbox policy — run directly on host.
		slog.Warn("sandbox: no config provided, running command without Landlock enforcement", "command", req.Command)
		return hostExec(ctx, req)
	}

	exe, err := os.Executable()
	if err != nil {
		return hostExec(ctx, req)
	}

	reexecArgs := buildReexecArgs(req.Shell, []string{req.ShellFlag, req.Command}, req.Config)

	cmd := exec.CommandContext(ctx, exe, reexecArgs...)
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

loop:
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
			break loop
		}
	}

	if len(cmdArgs) < 3 {
		fmt.Fprintln(os.Stderr, "cobot-sandbox: missing command")
		os.Exit(1)
	}

	applyLandlock(root, allowPaths, roPaths, noNetwork)

	// Resolve the executable path. unix.Exec does not search $PATH.
	shell := cmdArgs[0]
	shellPath, err := exec.LookPath(shell)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cobot-sandbox: lookpath %q: %v\n", shell, err)
		os.Exit(1)
	}

	// Replace this process with the shell command.
	// Using unix.Exec (not exec.Command) avoids a wrapper process that could
	// survive if the parent cancels the context.
	if err := unix.Exec(shellPath, cmdArgs, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "cobot-sandbox: exec: %v\n", err)
		os.Exit(1)
	}
	return true // unreachable
}

// applyLandlock applies filesystem and network restrictions using the Linux
// Landlock LSM. BestEffort mode gracefully degrades on kernels without Landlock
// support (older kernels, custom builds without CONFIG_LANDLOCK).
func applyLandlock(root string, allowPaths, roPaths []string, noNetwork bool) {
	cfg := landlock.V8.BestEffort()

	var rules []landlock.Rule

	// System paths are always readable (binaries, libraries, configs,
	// and minimal pseudo-filesystems commonly needed by shells and utilities).
	rules = append(rules, landlock.RODirs("/usr", "/bin", "/sbin", "/lib", "/lib64", "/etc", "/dev", "/proc"))

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
