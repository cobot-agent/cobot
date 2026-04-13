package builtin

import (
	"path/filepath"
	"strings"
)

type SandboxChecker interface {
	IsAllowed(path string, write bool) bool
}

type WorkspaceSandbox struct {
	Root          string
	AllowPaths    []string
	ReadonlyPaths []string
}

func (s *WorkspaceSandbox) IsAllowed(path string, write bool) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, p := range s.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return !write
		}
	}

	for _, p := range s.AllowPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return true
		}
	}

	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		if isSubpath(absPath, absRoot) {
			return true
		}
	}

	return false
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}
