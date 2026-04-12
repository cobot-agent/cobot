package builtin

import (
	"path/filepath"
	"strings"
)

type SandboxChecker interface {
	IsAllowed(path string, write bool) bool
}

type WorkspaceSandbox struct {
	Root       string
	AllowPaths []string
}

func (s *WorkspaceSandbox) IsAllowed(path string, write bool) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		if rel, err := filepath.Rel(absRoot, absPath); err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	for _, p := range s.AllowPaths {
		absP, _ := filepath.Abs(p)
		if rel, err := filepath.Rel(absP, absPath); err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}
