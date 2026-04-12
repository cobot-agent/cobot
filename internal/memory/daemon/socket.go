package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
)

func SocketPath(dataDir string) string {
	return filepath.Join(dataDir, "memory.sock")
}

func PidPath(dataDir string) string {
	return filepath.Join(dataDir, "memoryd.pid")
}

func IsRunning(ctx context.Context, socketPath string) bool {
	dialer := net.Dialer{Timeout: 100 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return false
	}
	defer conn.Close()

	cli := jrpc2.NewClient(channel.Line(conn, conn), nil)
	defer cli.Close()

	var result string
	err = cli.CallResult(ctx, "memory.ping", nil, &result)
	return err == nil && result == "pong"
}

func waitForSocket(ctx context.Context, socketPath string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		if _, err := os.Stat(socketPath); err == nil {
			if IsRunning(ctx, socketPath) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
