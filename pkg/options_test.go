package cobot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if cfg.Model != "openai:gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "openai:gpt-4o")
	}
	if cfg.APIKeys == nil {
		t.Error("APIKeys is nil, want non-nil map")
	}
	if !cfg.Memory.Enabled {
		t.Error("Memory.Enabled = false, want true")
	}
	if !cfg.Memory.IntelligentCuration {
		t.Error("Memory.IntelligentCuration = false, want true")
	}
	if cfg.Memory.CurationInterval.Seconds() != 30 {
		t.Errorf("Memory.CurationInterval = %v, want 30s", cfg.Memory.CurationInterval)
	}
}

func TestSandboxConfig_IsAllowed(t *testing.T) {
	root := t.TempDir()
	allowedDir := filepath.Join(root, "allowed")
	readonlyDir := filepath.Join(root, "readonly")
	os.MkdirAll(allowedDir, 0755)
	os.MkdirAll(readonlyDir, 0755)

	s := &SandboxConfig{
		Root:          root,
		AllowPaths:    []string{allowedDir},
		ReadonlyPaths: []string{readonlyDir},
	}

	allowedFile := filepath.Join(allowedDir, "file.txt")
	readonlyFile := filepath.Join(readonlyDir, "file.txt")
	rootFile := filepath.Join(root, "file.txt")
	outsideFile := filepath.Join(os.TempDir(), "outside.txt")

	if !s.IsAllowed(allowedFile, false) {
		t.Error("allowed path should be readable")
	}
	if !s.IsAllowed(allowedFile, true) {
		t.Error("allowed path should be writable")
	}
	if !s.IsAllowed(readonlyFile, false) {
		t.Error("readonly path should be readable")
	}
	if s.IsAllowed(readonlyFile, true) {
		t.Error("readonly path should not be writable")
	}
	if !s.IsAllowed(rootFile, false) {
		t.Error("root path should be readable")
	}
	if !s.IsAllowed(rootFile, true) {
		t.Error("root path should be writable")
	}
	if s.IsAllowed(outsideFile, false) {
		t.Error("path outside root should not be allowed")
	}
}

func TestSandboxConfig_IsBlockedCommand(t *testing.T) {
	s := &SandboxConfig{
		BlockedCommands: []string{"rm -rf", "format", "dd if="},
	}

	if !s.IsBlockedCommand("rm -rf /") {
		t.Error("should block rm -rf")
	}
	if !s.IsBlockedCommand("format C:") {
		t.Error("should block format")
	}
	if s.IsBlockedCommand("ls -la") {
		t.Error("should not block ls")
	}
	if !s.IsBlockedCommand("dd if=/dev/zero of=/dev/sda") {
		t.Error("should block dd")
	}
}
