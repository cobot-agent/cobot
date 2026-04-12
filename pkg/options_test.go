package cobot

import (
	"path/filepath"
	"testing"
	"time"
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
		t.Error("APIKeys is nil")
	}
	if !cfg.Memory.Enabled {
		t.Error("Memory.Enabled = false, want true")
	}
	if !cfg.Memory.IntelligentCuration {
		t.Error("Memory.IntelligentCuration = false, want true")
	}
	if cfg.Memory.CurationInterval != 30*time.Second {
		t.Errorf("Memory.CurationInterval = %v, want 30s", cfg.Memory.CurationInterval)
	}
	if cfg.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty", cfg.ConfigPath)
	}
	if cfg.DataPath != "" {
		t.Errorf("DataPath = %q, want empty", cfg.DataPath)
	}
}

func TestSandboxConfig_IsAllowed_AllowPaths(t *testing.T) {
	tmp := t.TempDir()
	s := &SandboxConfig{
		AllowPaths: []string{tmp},
	}
	if !s.IsAllowed(filepath.Join(tmp, "file.txt"), true) {
		t.Error("expected allowed path to be writable")
	}
	if !s.IsAllowed(filepath.Join(tmp, "file.txt"), false) {
		t.Error("expected allowed path to be readable")
	}
	if s.IsAllowed("/nonexistent/path/file.txt", true) {
		t.Error("expected unrelated path to be denied")
	}
}

func TestSandboxConfig_IsAllowed_ReadonlyPaths(t *testing.T) {
	tmp := t.TempDir()
	s := &SandboxConfig{
		ReadonlyPaths: []string{tmp},
	}
	if !s.IsAllowed(filepath.Join(tmp, "file.txt"), false) {
		t.Error("expected readonly path to be readable")
	}
	if s.IsAllowed(filepath.Join(tmp, "file.txt"), true) {
		t.Error("expected readonly path to NOT be writable")
	}
}

func TestSandboxConfig_IsAllowed_Root(t *testing.T) {
	tmp := t.TempDir()
	s := &SandboxConfig{
		Root: tmp,
	}
	if !s.IsAllowed(filepath.Join(tmp, "sub", "file.txt"), true) {
		t.Error("expected root subpath to be allowed")
	}
}

func TestSandboxConfig_IsAllowed_NoConfig(t *testing.T) {
	s := &SandboxConfig{}
	if s.IsAllowed("/some/path", true) {
		t.Error("expected denied with empty config")
	}
}

func TestSandboxConfig_IsBlockedCommand(t *testing.T) {
	s := &SandboxConfig{
		BlockedCommands: []string{"rm -rf", "mkfs", "dd if="},
	}
	if !s.IsBlockedCommand("rm -rf /") {
		t.Error("expected 'rm -rf /' to be blocked")
	}
	if !s.IsBlockedCommand("sudo mkfs.ext4 /dev/sda1") {
		t.Error("expected mkfs command to be blocked")
	}
	if s.IsBlockedCommand("ls -la") {
		t.Error("expected 'ls -la' to NOT be blocked")
	}
}

func TestSandboxConfig_IsBlockedCommand_Empty(t *testing.T) {
	s := &SandboxConfig{}
	if s.IsBlockedCommand("rm -rf /") {
		t.Error("expected no blocks with empty config")
	}
}

func TestIsSubpath(t *testing.T) {
	tmp := t.TempDir()
	if !isSubpath(filepath.Join(tmp, "a", "b"), tmp) {
		t.Error("expected subpath")
	}
	if isSubpath("/a/b", "/x/y") {
		t.Error("expected non-subpath")
	}
}
