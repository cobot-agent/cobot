package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceSandbox_IsAllowed(t *testing.T) {
	dir := t.TempDir()
	sandbox := &WorkspaceSandbox{Root: dir}

	allowed := filepath.Join(dir, "sub", "file.txt")
	if !sandbox.IsAllowed(allowed, false) {
		t.Error("expected path inside root to be allowed")
	}

	if sandbox.IsAllowed("/etc/passwd", false) {
		t.Error("expected path outside root to be blocked")
	}
}

func TestWorkspaceSandbox_AllowPaths(t *testing.T) {
	dir := t.TempDir()
	extra := t.TempDir()
	sandbox := &WorkspaceSandbox{Root: dir, AllowPaths: []string{extra}}

	extraFile := filepath.Join(extra, "data.txt")
	if !sandbox.IsAllowed(extraFile, false) {
		t.Error("expected AllowPaths entry to be allowed")
	}
}

func TestReadFileTool_SandboxAllowed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	sandbox := &WorkspaceSandbox{Root: dir}
	tool := NewReadFileTool(WithReadSandbox(sandbox))
	args, _ := json.Marshal(map[string]string{"path": f})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %s", result)
	}
}

func TestReadFileTool_SandboxBlocked(t *testing.T) {
	dir := t.TempDir()
	sandbox := &WorkspaceSandbox{Root: dir}
	tool := NewReadFileTool(WithReadSandbox(sandbox))
	args, _ := json.Marshal(map[string]string{"path": "/etc/passwd"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for path outside sandbox")
	}
}

func TestWriteFileTool_SandboxBlocked(t *testing.T) {
	dir := t.TempDir()
	sandbox := &WorkspaceSandbox{Root: dir}
	tool := NewWriteFileTool(WithWriteSandbox(sandbox))
	args, _ := json.Marshal(map[string]string{"path": "/tmp/outside_sandbox.txt", "content": "nope"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for write outside sandbox")
	}
}

func TestShellExecTool_BlockedCommand(t *testing.T) {
	tool := NewShellExecTool(WithShellSandbox("", []string{"rm", "mkfs"}))
	args, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for blocked command")
	}
}

func TestShellExecTool_Workdir(t *testing.T) {
	dir := t.TempDir()
	tool := NewShellExecTool(WithShellSandbox(dir, nil))
	args, _ := json.Marshal(map[string]string{"command": "pwd"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	expected := dir + "\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestReadFileTool_NoSandboxBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "compat.txt")
	os.WriteFile(f, []byte("compat"), 0644)

	tool := NewReadFileTool()
	args, _ := json.Marshal(map[string]string{"path": f})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "compat" {
		t.Errorf("expected 'compat', got %s", result)
	}
}

func TestWriteFileTool_NoSandboxBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "compat.txt")

	tool := NewWriteFileTool()
	args, _ := json.Marshal(map[string]string{"path": f, "content": "compat"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Errorf("expected ok, got %s", result)
	}
}

func TestShellExecTool_NoSandboxBackwardCompat(t *testing.T) {
	tool := NewShellExecTool()
	args, _ := json.Marshal(map[string]string{"command": "echo compat"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "compat\n" {
		t.Errorf("expected 'compat\\n', got %q", result)
	}
}
