package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgentConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "helper.yaml")

	content := `name: code-helper
model: gpt-4
system_prompt: "You are a helpful coding assistant."
enabled_mcp:
  - filesystem
  - github
enabled_skills:
  - debugging
  - refactoring
max_turns: 100
sandbox:
  root: /workspace
  allow_paths:
    - /workspace/src
    - /workspace/test
  blocked_commands:
    - rm -rf
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}

	if cfg.Name != "code-helper" {
		t.Errorf("Name = %q, want %q", cfg.Name, "code-helper")
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4")
	}
	if cfg.SystemPrompt != "You are a helpful coding assistant." {
		t.Errorf("SystemPrompt = %q, want %q", cfg.SystemPrompt, "You are a helpful coding assistant.")
	}
	if len(cfg.EnabledMCP) != 2 || cfg.EnabledMCP[0] != "filesystem" || cfg.EnabledMCP[1] != "github" {
		t.Errorf("EnabledMCP = %v, want [filesystem github]", cfg.EnabledMCP)
	}
	if len(cfg.EnabledSkills) != 2 || cfg.EnabledSkills[0] != "debugging" || cfg.EnabledSkills[1] != "refactoring" {
		t.Errorf("EnabledSkills = %v, want [debugging refactoring]", cfg.EnabledSkills)
	}
	if cfg.MaxTurns != 100 {
		t.Errorf("MaxTurns = %d, want 100", cfg.MaxTurns)
	}
	if cfg.Sandbox == nil {
		t.Fatal("Sandbox is nil")
	}
	if cfg.Sandbox.Root != "/workspace" {
		t.Errorf("Sandbox.Root = %q, want %q", cfg.Sandbox.Root, "/workspace")
	}
	if len(cfg.Sandbox.AllowPaths) != 2 {
		t.Errorf("Sandbox.AllowPaths = %v, want 2 entries", cfg.Sandbox.AllowPaths)
	}
	if len(cfg.Sandbox.BlockedCommands) != 1 || cfg.Sandbox.BlockedCommands[0] != "rm -rf" {
		t.Errorf("Sandbox.BlockedCommands = %v, want [rm -rf]", cfg.Sandbox.BlockedCommands)
	}
}

func TestLoadAgentConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "my-agent.yaml")

	content := `model: gpt-3.5-turbo
system_prompt: "Hello"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}

	if cfg.Name != "my-agent" {
		t.Errorf("Name = %q, want %q (should default to filename without extension)", cfg.Name, "my-agent")
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50 (default)", cfg.MaxTurns)
	}
	if cfg.Sandbox != nil {
		t.Errorf("Sandbox should be nil when not set, got %v", cfg.Sandbox)
	}
}

func TestLoadAgentConfigsFromDir(t *testing.T) {
	dir := t.TempDir()

	cfg1 := `name: agent-one
model: gpt-4
system_prompt: "First agent"
max_turns: 30
`
	cfg2 := `name: agent-two
model: claude-3
system_prompt: "Second agent"
`
	if err := os.WriteFile(filepath.Join(dir, "one.yaml"), []byte(cfg1), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "two.yaml"), []byte(cfg2), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	configs, err := LoadAgentConfigs(dir)
	if err != nil {
		t.Fatalf("LoadAgentConfigs() error = %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	if cfg, ok := configs["agent-one"]; !ok {
		t.Error("missing agent-one")
	} else {
		if cfg.Model != "gpt-4" {
			t.Errorf("agent-one Model = %q, want gpt-4", cfg.Model)
		}
		if cfg.MaxTurns != 30 {
			t.Errorf("agent-one MaxTurns = %d, want 30", cfg.MaxTurns)
		}
	}

	if cfg, ok := configs["agent-two"]; !ok {
		t.Error("missing agent-two")
	} else {
		if cfg.Model != "claude-3" {
			t.Errorf("agent-two Model = %q, want claude-3", cfg.Model)
		}
		if cfg.MaxTurns != 50 {
			t.Errorf("agent-two MaxTurns = %d, want 50 (default)", cfg.MaxTurns)
		}
	}
}

func TestLoadAgentConfigs_NonexistentDir(t *testing.T) {
	configs, err := LoadAgentConfigs("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("LoadAgentConfigs() error = %v, want nil for nonexistent dir", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected empty map, got %d entries", len(configs))
	}
}
