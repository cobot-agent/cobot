//go:build !linux
// +build !linux

package sandbox

// isUnshareAvailable always returns false on non-Linux platforms.
func isUnshareAvailable() bool {
	return false
}
