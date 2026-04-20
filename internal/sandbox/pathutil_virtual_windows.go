//go:build windows

package sandbox

import (
	"path/filepath"
	"strings"
)

func VirtualHome(name string) string {
	return filepath.Join(`C:\Users`, name)
}

func VirtualSeparator() string {
	return `\`
}

func VirtualToNative(path string) string {
	return strings.ReplaceAll(path, "/", `\`)
}
