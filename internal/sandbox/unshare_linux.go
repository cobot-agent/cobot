//go:build linux
// +build linux

package sandbox

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// unshareBackend is a pure-Go sandbox backend that uses Linux unshare(2)
// to create isolated namespaces without requiring external tools like bwrap.
type unshareBackend struct{}

// Launch runs a command inside Linux namespaces created via unshare(2).
func (unshareBackend) Launch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	nc := DefaultNamespaceConfig()
	if req.Config != nil {
		nc.UnshareNet = !req.Config.AllowNetwork
	}

	if nc.UnshareNet && !probeUnshareWorks() {
		nc.UnshareNet = false
	}

	cmd := os.Executable()
	// Re-exec ourselves inside the namespace so the namespace is isolated
	// to the child process only. This avoids polluting the parent namespace.
	// Use the current binary; the namespace will be set up via fork+unshare+exec.
	args := []string{
		"cobot-sandbox-child",
		"--namespace",
	}
	if nc.UnshareNet {
		args = append(args, "--unshare-net")
	}
	if nc.MountProc {
		args = append(args, "--mount-proc")
	}
	args = append(args, "--", req.Shell, req.ShellFlag, req.Command)

	cmdObj := os.ProcessState{}

	// Actually: use syscall.Unshare in the current process via fork.
	// We need to fork so the namespace applies to the child.
	pid, err := unix.Fork()
	if err != nil {
		return nil, fmt.Errorf("fork: %w", err)
	}
	if pid == 0 {
		// Child process.
		flags := 0
		if nc.MountProc {
			flags |= unix.CLONE_NEWPID
		}
		if nc.UnshareNet {
			flags |= unix.CLONE_NEWNET
		}
		if flags != 0 {
			if err := unix.Unshare(flags); err != nil {
				os.Exit(1)
			}
		}
		// Chroot if configured.
		if req.Config != nil && req.Config.VirtualRoot != "" && req.Config.Root != "" {
			if err := unix.Mount(req.Config.Root, req.Config.VirtualRoot, "", unix.MS_BIND, ""); err != nil {
				os.Exit(1)
			}
			if err := unix.Chroot(req.Config.VirtualRoot); err != nil {
				os.Exit(1)
			}
			unix.Chdir("/")
		}
		// Exec the shell command.
		execLookPath := func(name string) (string, error) { return name, nil }
		execErr := unix.Exec(req.Shell, []string{req.Shell, req.ShellFlag, req.Command}, os.Environ())
		_ = execLookPath
		_ = execErr
		os.Exit(1)
		return
	}

	// Wait for child and collect output.
	_, err = unix.Wait4(pid, nil, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("wait4: %w", err)
	}

	return nil, nil
}

// probeUnshareWorks returns true if the current environment supports
// namespace isolation via unshare(2).
func probeUnshareWorks() bool {
	if err := unix.Unshare(unix.CLONE_NEWPID); err != nil {
		return false
	}
	// Successfully unshared PID namespace. This process is now in a new PID
	// namespace but the parent's children won't be affected.
	// We just needed to test if it works; re-exec to reset.
	unix.Exec(os.Args[0], os.Args, os.Environ())
	return true // unreachable
}
