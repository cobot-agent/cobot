package builtin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceSandbox_PathUnderRoot(t *testing.T) {
	dir := t.TempDir()
	s := &WorkspaceSandbox{Root: dir}

	if !s.IsAllowed(filepath.Join(dir, "foo.txt"), false) {
		t.Error("read under root should be allowed")
	}
	if !s.IsAllowed(filepath.Join(dir, "foo.txt"), true) {
		t.Error("write under root should be allowed")
	}
}

func TestWorkspaceSandbox_PathUnderAllowPaths(t *testing.T) {
	dir := t.TempDir()
	s := &WorkspaceSandbox{Root: "/nonexistent", AllowPaths: []string{dir}}

	if !s.IsAllowed(filepath.Join(dir, "bar.txt"), false) {
		t.Error("read under allow path should be allowed")
	}
	if !s.IsAllowed(filepath.Join(dir, "bar.txt"), true) {
		t.Error("write under allow path should be allowed")
	}
}

func TestWorkspaceSandbox_PathUnderReadonlyPaths(t *testing.T) {
	dir := t.TempDir()
	s := &WorkspaceSandbox{Root: "/nonexistent", ReadonlyPaths: []string{dir}}

	if !s.IsAllowed(filepath.Join(dir, "baz.txt"), false) {
		t.Error("read under readonly path should be allowed")
	}
	if s.IsAllowed(filepath.Join(dir, "baz.txt"), true) {
		t.Error("write under readonly path should be denied")
	}
}

func TestWorkspaceSandbox_PathOutsideAll(t *testing.T) {
	s := &WorkspaceSandbox{Root: "/nonexistent"}

	if s.IsAllowed("/tmp/outside.txt", false) {
		t.Error("read outside all paths should be denied")
	}
	if s.IsAllowed("/tmp/outside.txt", true) {
		t.Error("write outside all paths should be denied")
	}
}

func TestWorkspaceSandbox_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	s := &WorkspaceSandbox{Root: dir}

	traversal := filepath.Join(dir, "..", "..", "etc", "passwd")
	if s.IsAllowed(traversal, false) {
		t.Error("path traversal outside root should be denied")
	}
}

func TestWorkspaceSandbox_EmptyRoot(t *testing.T) {
	s := &WorkspaceSandbox{Root: ""}

	if s.IsAllowed("/some/path", false) {
		t.Error("with empty root, no path should be allowed")
	}
	if s.IsAllowed("/some/path", true) {
		t.Error("with empty root, no path should be allowed")
	}
}

func TestWorkspaceSandbox_ReadonlyTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	s := &WorkspaceSandbox{
		Root:          dir,
		ReadonlyPaths: []string{dir},
	}

	if !s.IsAllowed(filepath.Join(dir, "file.txt"), false) {
		t.Error("read under readonly+root should be allowed")
	}
	if s.IsAllowed(filepath.Join(dir, "file.txt"), true) {
		t.Error("write under readonly+root should be denied (readonly takes precedence)")
	}
}

func TestWorkspaceSandbox_Subdirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub", "deep")
	os.MkdirAll(sub, 0755)
	s := &WorkspaceSandbox{Root: dir}

	if !s.IsAllowed(filepath.Join(sub, "file.txt"), true) {
		t.Error("deep subdirectory under root should be allowed")
	}
}
