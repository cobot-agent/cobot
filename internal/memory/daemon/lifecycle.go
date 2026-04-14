package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cobot-agent/cobot/internal/memory"
)

func StartOrConnect(ctx context.Context, dataDir string) (memory.Client, func(), error) {
	socketPath := SocketPath(dataDir)

	if IsRunning(ctx, socketPath) {
		rs, err := Dial(ctx, socketPath)
		if err == nil {
			slog.Debug("memory: connected to existing daemon", "socket", socketPath)
			return rs, func() { rs.Close() }, nil
		}
	}

	if err := startDaemon(dataDir); err != nil {
		slog.Debug("memory: daemon start failed, falling back to direct store", "error", err)
		return fallbackStore(dataDir)
	}

	if err := waitForSocket(ctx, socketPath, 5*time.Second); err != nil {
		slog.Debug("memory: daemon socket timeout, falling back to direct store", "error", err)
		return fallbackStore(dataDir)
	}

	rs, err := Dial(ctx, socketPath)
	if err != nil {
		slog.Debug("memory: daemon dial failed, falling back to direct store", "error", err)
		return fallbackStore(dataDir)
	}

	slog.Debug("memory: connected to new daemon", "socket", socketPath)
	return rs, func() { rs.Close() }, nil
}

func startDaemon(dataDir string) error {
	os.Remove(SocketPath(dataDir))

	binary, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(binary, "memoryd", "--data", dataDir)
	cmd.SysProcAttr = sysProcAttr()
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	pidPath := PidPath(dataDir)
	os.MkdirAll(filepath.Dir(pidPath), 0755)
	os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	return nil
}

func fallbackStore(dataDir string) (memory.Client, func(), error) {
	memDir := filepath.Join(dataDir, "memory")
	store, err := memory.OpenStore(memDir)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open direct store: %w", err)
	}
	return store, func() { store.Close() }, nil
}

func ServeMemoryDaemon(ctx context.Context, dataDir string) error {
	memDir := filepath.Join(dataDir, "memory")
	store, err := memory.OpenStore(memDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	socketPath := SocketPath(dataDir)
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer os.Remove(socketPath)
	os.Chmod(socketPath, 0600)

	slog.Info("memoryd: listening", "socket", socketPath)

	srv := NewServer(store)
	return srv.Serve(ctx, listener)
}
