// Package util provides shared internal utility functions for the cobot project.
package util

import (
	"path/filepath"
	"runtime"
	"strings"
)

// EvalSymlinks resolves symlinks in the given path. If the full path cannot be
// resolved (e.g. it points to a non-existent location), it walks up parent
// directories until it finds one that can be resolved and joins the remainder.
// On Windows, symlinks are handled by filepath.EvalSymlinks which accounts for
// junction points and NTFS symlinks. The function falls back to returning the
// original path unchanged if no component can be resolved.
func EvalSymlinks(path string) string {
	realPath, err := filepath.EvalSymlinks(path)
	if err == nil {
		return realPath
	}

	dir := filepath.Dir(path)
	tail := filepath.Base(path)

	// Walk up the directory tree until we find a resolvable ancestor.
	for len(dir) > 0 && dir != "/" && dir != "." {
		// On Windows, also stop at volume root (e.g. "C:\")
		if runtime.GOOS == "windows" {
			vol := filepath.VolumeName(dir)
			if vol != "" && dir == vol+"\\" {
				break
			}
		}

		realDir, err := filepath.EvalSymlinks(dir)
		if err == nil {
			return filepath.Join(realDir, tail)
		}
		tail = filepath.Base(dir) + string(filepath.Separator) + tail
		dir = filepath.Dir(dir)
	}

	return path
}

// IsSubpath reports whether path is equal to or a descendant of base.
// It uses filepath.Rel and checks that the relative path does not escape
// upward (i.e. does not start with "..").
func IsSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
