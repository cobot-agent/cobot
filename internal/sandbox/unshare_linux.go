//go:build linux
// +build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// unshareBackend is a pure-Go sandbox backend that uses Linux unshare(2)
// to create isolated namespaces without requiring external tools like bwrap.
type unshareBackend struct{}

// Launch runs a command inside Linux namespaces created via unshare(2).
// It uses a re-execution model: the current binary is re-executed with a
// special child-mode argument, and the child calls unshare(2) before
// running the actual shell command. This avoids the need for raw fork(2)
// in Go (which is unsafe due to the runtime).
func (unshareBackend) Launch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("launch request is required")
	}

	nc := DefaultNamespaceConfig()
	if req.Config != nil {
		nc.UnshareNet = !req.Config.AllowNetwork
	}

	// Probe whether unshare works in this environment (e.g. CI may lack
	// CAP_SYS_ADMIN). If not, fall back to hostBackend behavior.
	if nc.UnshareNet && !probeUnshareWorks() {
		nc.UnshareNet = false
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot find executable: %w", err)
	}

	// Build child-mode arguments. The child process will see these and
	// enter the requested namespaces before exec'ing the shell.
	args := []string{exe, "__cobot_sandbox_child__"}
	if nc.UnshareNet {
		args = append(args, "--unshare-net")
	}
	if nc.MountProc {
		args = append(args, "--mount-proc")
	}
	if req.Config != nil && req.Config.VirtualRoot != "" {
		args = append(args, "--chroot", req.Config.VirtualRoot, req.Config.Root)
	}
	args = append(args, "--")
	args = append(args, req.Shell, req.ShellFlag, req.Command)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}

	return cmd.CombinedOutput()
}

// probeUnshareWorks returns true if the current environment supports
// namespace isolation via unshare(2). It attempts a lightweight test
// unshare and checks for permission errors.
func probeUnshareWorks() bool {
	// Test with CLONE_NEWIPC (lightweight, no need to re-exec).
	if err := unix.Unshare(unix.CLONE_NEWIPC); err != nil {
		return false
	}
	// We successfully created a new IPC namespace. This process is now
	// in it, but that doesn't affect anything meaningful for our use.
	// No need to re-exec; just return true.
	return true
}

// HandleSandboxChildMode checks if the current process was invoked in
// sandbox child mode (via re-exec from unshareBackend) and if so,
// enters the requested namespaces and execs the target command.
//
// This should be called early in main() before any other setup.
// It returns true if child mode was handled (caller should exit),
// false if normal startup should continue.
func HandleSandboxChildMode() bool {
	if len(os.Args) < 2 || os.Args[1] != "__cobot_sandbox_child__" {
		return false
	}

	args := os.Args[2:]
	var (
		unshareNet bool
		mountProc  bool
		chrootVr   string
		chrootRoot string
		cmdArgs    []string
	)

	// Parse child-mode flags.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--unshare-net":
			unshareNet = true
		case "--mount-proc":
			mountProc = true
		case "--chroot":
			if i+2 < len(args) {
				chrootVr = args[i+1]
				chrootRoot = args[i+2]
				i += 2
			}
		case "--":
			cmdArgs = args[i+1:]
			break
		}
	}

	if len(cmdArgs) < 3 {
		fmt.Fprintln(os.Stderr, "cobot-sandbox-child: missing command")
		os.Exit(1)
	}

	// Build unshare flags.
	flags := 0
	if mountProc {
		flags |= unix.CLONE_NEWPID | unix.CLONE_NEWNS
	}
	if unshareNet {
		flags |= unix.CLONE_NEWNET
	}
	if flags != 0 {
		if err := unix.Unshare(flags); err != nil {
			fmt.Fprintf(os.Stderr, "cobot-sandbox-child: unshare: %v\n", err)
			os.Exit(1)
		}
	}

	// Chroot if configured.
	if chrootVr != "" && chrootRoot != "" {
		if err := unix.Mount(chrootRoot, chrootVr, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			fmt.Fprintf(os.Stderr, "cobot-sandbox-child: bind mount: %v\n", err)
			os.Exit(1)
		}
		if err := unix.Chroot(chrootVr); err != nil {
			fmt.Fprintf(os.Stderr, "cobot-sandbox-child: chroot: %v\n", err)
			os.Exit(1)
		}
		unix.Chdir("/")
	}

	// Mount /proc in the new PID namespace if requested.
	if mountProc {
		if err := unix.Mount("proc", "/proc", "proc", 0, ""); err != nil {
			fmt.Fprintf(os.Stderr, "cobot-sandbox-child: mount /proc: %v\n", err)
			os.Exit(1)
		}
	}

	// Exec the shell command. This replaces the current process image.
	err := unix.Exec(cmdArgs[0], cmdArgs, os.Environ())
	fmt.Fprintf(os.Stderr, "cobot-sandbox-child: exec: %v\n", err)
	os.Exit(1)
	return true // unreachable
}

// isUnshareAvailable reports whether unshare(2) is available and usable.
func isUnshareAvailable() bool {
	return probeUnshareWorks()
}
