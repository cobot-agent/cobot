//go:build windows

package sandbox

import (
	"context"
)

func platformLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	return restrictedTokenLaunch(ctx, req)
}
