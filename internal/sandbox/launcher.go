package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Backend is the interface for sandbox execution backends.
type Backend interface {
	Launch(context.Context, *LaunchRequest) ([]byte, error)
}

// LaunchRequest contains the parameters for launching a sandboxed command.
type LaunchRequest struct {
	Shell     string
	ShellFlag string
	Command   string
	Dir       string
	Config    *SandboxConfig
}

// Launcher selects and uses a Backend to run commands in a sandbox.
type Launcher struct {
	backend       Backend
	sandboxConfig *SandboxConfig
}

// defaultBackend returns the best available backend for the current platform.
func defaultBackend() Backend {
	return hostBackend{}
}

// LauncherOption configures a Launcher.
type LauncherOption func(*Launcher)

// WithBackend sets the backend used by a Launcher.
func WithBackend(backend Backend) LauncherOption {
	return func(l *Launcher) {
		if backend != nil {
			l.backend = backend
		}
	}
}

// WithSandboxConfig sets the sandbox configuration for a Launcher.
func WithSandboxConfig(cfg *SandboxConfig) LauncherOption {
	return func(l *Launcher) {
		l.sandboxConfig = cfg
	}
}

// NewLauncher creates a Launcher with the given options.
func NewLauncher(opts ...LauncherOption) *Launcher {
	launcher := &Launcher{backend: defaultBackend()}
	for _, opt := range opts {
		opt(launcher)
	}
	if launcher.backend == nil {
		launcher.backend = hostBackend{}
	}
	return launcher
}

// Launch runs a command using the configured backend.
func (l *Launcher) Launch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("launch request is required")
	}
	if l == nil {
		l = NewLauncher()
	}

	backend := l.backend
	if backend == nil {
		backend = hostBackend{}
	}

	return backend.Launch(ctx, req)
}

// hostBackend runs commands directly on the host.
type hostBackend struct{}

func (hostBackend) Launch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("launch request is required")
	}
	cmd := exec.CommandContext(ctx, req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}

// isCI reports whether the process appears to be running in a CI environment.
func isCI() bool {
	return os.Getenv("CI") == "true"
}
