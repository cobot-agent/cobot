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
	realPath, symErr := filepath.EvalSymlinks(absPath)
	if symErr == nil {
		absPath = realPath
	}

	readonlyMatched := false
	for _, p := range s.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		realP, symErr := filepath.EvalSymlinks(absP)
		if symErr == nil {
			absP = realP
		}
		if isSubpath(absPath, absP) {
			readonlyMatched = true
			if write {
				return false
			}
		}
	}

	for _, p := range s.AllowPaths {
		absP, _ := filepath.Abs(p)
		realP, symErr := filepath.EvalSymlinks(absP)
		if symErr == nil {
			absP = realP
		}
		if isSubpath(absPath, absP) {
			if readonlyMatched && write {
				return false
			}
			return true
		}
	}

	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		realRoot, symErr := filepath.EvalSymlinks(absRoot)
		if symErr == nil {
			absRoot = realRoot
		}
		if isSubpath(absPath, absRoot) {
			if readonlyMatched && write {
				return false
			}
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
