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

func evalSymlinks(path string) string {
	realPath, err := filepath.EvalSymlinks(path)
	if err == nil {
		return realPath
	}
	dir := filepath.Dir(path)
	tail := filepath.Base(path)
	for len(dir) > 0 && dir != "/" {
		realDir, err := filepath.EvalSymlinks(dir)
		if err == nil {
			return filepath.Join(realDir, tail)
		}
		tail = filepath.Base(dir) + "/" + tail
		dir = filepath.Dir(dir)
	}
	return path
}

func (s *WorkspaceSandbox) IsAllowed(path string, write bool) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absPath = evalSymlinks(absPath)

	readonlyMatched := false
	for _, p := range s.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		absP = evalSymlinks(absP)
		if isSubpath(absPath, absP) {
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
		absP = evalSymlinks(absP)
		if isSubpath(absPath, absP) {
			if readonlyMatched && write {
				return false
			}
			return true
		}
	}

	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		absRoot = evalSymlinks(absRoot)
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
