//go:build windows

package main

import (
	"os"
)

// InterruptSignals returns the OS signals to trap for graceful shutdown.
// On Windows, SIGTERM is not available, so only os.Interrupt is used.
func InterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
