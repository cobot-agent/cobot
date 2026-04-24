//go:build darwin

package sandbox

import "context"

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return sandboxExecLaunch(ctx, req)
}
