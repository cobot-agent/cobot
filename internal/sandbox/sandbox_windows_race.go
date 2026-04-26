//go:build windows && race

package sandbox

import (
	"context"
	"log/slog"
)

// restrictedTokenLaunch is a no-op fallback when built with -race.
// CreateRestrictedToken + race detector causes STATUS_HEAP_CORRUPTION
// (0xc0000374) on Windows, so we fall back to unsandboxed execution.
func restrictedTokenLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	slog.Warn("sandbox: built with -race, Restricted Token enforcement disabled")
	return hostExec(ctx, req)
}
