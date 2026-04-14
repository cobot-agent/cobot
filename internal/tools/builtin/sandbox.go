package builtin

import (
	"path/filepath"

	"github.com/cobot-agent/cobot/internal/util"
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
	absPath = util.EvalSymlinks(absPath)

	readonlyMatched := false
	for _, p := range s.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		absP = util.EvalSymlinks(absP)
		if util.IsSubpath(absPath, absP) {
			if write {
				return false
			}
			readonlyMatched = true
		}
	}
	if readonlyMatched {
		return true
	}

	for _, p := range s.AllowPaths {
		absP, _ := filepath.Abs(p)
		absP = util.EvalSymlinks(absP)
		if util.IsSubpath(absPath, absP) {
			return true
		}
	}

	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		absRoot = util.EvalSymlinks(absRoot)
		if util.IsSubpath(absPath, absRoot) {
			return true
		}
	}

	return false
}
