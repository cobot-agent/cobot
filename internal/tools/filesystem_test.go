package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTool(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello world"), 0644)

	tool := NewReadFileTool()
	args, _ := json.Marshal(map[string]string{"path": f})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %s", result)
	}
}

func TestWriteFileTool(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "output.txt")

	tool := NewWriteFileTool()
	args, _ := json.Marshal(map[string]string{"path": f, "content": "written content"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Errorf("expected ok, got %s", result)
	}
	data, _ := os.ReadFile(f)
	if string(data) != "written content" {
		t.Errorf("file content mismatch: %s", string(data))
	}
}

func TestReadFileNotFound(t *testing.T) {
	tool := NewReadFileTool()
	args, _ := json.Marshal(map[string]string{"path": "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestShellExecTool(t *testing.T) {
	tool := NewShellExecTool()
	args, _ := json.Marshal(map[string]string{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	result = strings.ReplaceAll(result, "\r\n", "\n")
	if result != "hello\n" {
		t.Errorf("expected %q, got %q", "hello\n", result)
	}
}

func TestShellExecToolMultiArg(t *testing.T) {
	tool := NewShellExecTool()
	args, _ := json.Marshal(map[string]string{"command": "echo hello world"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	result = strings.ReplaceAll(result, "\r\n", "\n")
	if result != "hello world\n" {
		t.Errorf("expected %q, got %q", "hello world\n", result)
	}
}
