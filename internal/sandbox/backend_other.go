//go:build !linux && !darwin

package sandbox

import "context"

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return hostExec(ctx, req)
}
