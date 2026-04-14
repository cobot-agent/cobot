//go:build !windows

package main

import (
	"os"
	"syscall"
)

// InterruptSignals returns the OS signals to trap for graceful shutdown.
// On Unix, both SIGINT and SIGTERM trigger graceful shutdown so that
// container orchestrators (Docker, systemd) can stop the agent cleanly.
func InterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
