package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

var (
	rgPathOnce sync.Once
	rgPath     string
	rgPathErr  error

	fdPathOnce sync.Once
	fdPath     string
	fdPathErr  error
)

func findRg() (string, error) {
	rgPathOnce.Do(func() {
		rgPath, rgPathErr = exec.LookPath("rg")
	})
	return rgPath, rgPathErr
}

func findFd() (string, error) {
	fdPathOnce.Do(func() {
		// Try "fd" first (standard name), then "fdfind" (Debian/Ubuntu)
		fdPath, fdPathErr = exec.LookPath("fd")
		if fdPathErr != nil {
			fdPath, fdPathErr = exec.LookPath("fdfind")
		}
	})
	return fdPath, fdPathErr
}

const externalToolTimeout = 30 * time.Second

// runRg executes ripgrep with the given arguments and returns its stdout.
// Returns ("", error) if rg is not available or fails.
func runRg(ctx context.Context, args []string) (string, error) {
	rg, err := findRg()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, externalToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, rg, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("rg failed: %w (%s)", err, stderr.String())
	}
	return stdout.String(), nil
}

// runFd executes fd-find with the given arguments and returns its stdout.
// Returns ("", error) if fd is not available or fails.
func runFd(ctx context.Context, args []string) (string, error) {
	fd, err := findFd()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, externalToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, fd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("fd failed: %w (%s)", err, stderr.String())
	}
	return stdout.String(), nil
}
