//go:build !windows

package sandbox

import "path/filepath"

func VirtualHome(name string) string {
	return filepath.Join("/home", name)
}

func VirtualSeparator() string {
	return `/`
}

func VirtualToNative(path string) string {
	return path
}
