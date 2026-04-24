package sandbox

import (
	"context"
	"os/exec"
)

// hostExec runs a command directly on the host (no sandbox).
func hostExec(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	cmd := exec.CommandContext(ctx, req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	return cmd.CombinedOutput()
}
