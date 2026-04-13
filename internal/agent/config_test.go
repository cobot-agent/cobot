package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgentConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "assistant.yaml")

	content := []byte(`model: gpt-4
system_prompt: "You are helpful."
enabled_mcp:
  - fs
  - git
enabled_skills:
  - coding
max_turns: 30
sandbox:
  root: /workspace
  allow_paths:
    - /tmp
  blocked_commands:
    - rm -rf
`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig: %v", err)
	}

	if cfg.Name != "assistant" {
		t.Errorf("Name = %q, want %q", cfg.Name, "assistant")
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4")
	}
	if cfg.SystemPrompt != "You are helpful." {
		t.Errorf("SystemPrompt = %q, want %q", cfg.SystemPrompt, "You are helpful.")
	}
	if len(cfg.EnabledMCP) != 2 || cfg.EnabledMCP[0] != "fs" || cfg.EnabledMCP[1] != "git" {
		t.Errorf("EnabledMCP = %v, want [fs git]", cfg.EnabledMCP)
	}
	if len(cfg.EnabledSkills) != 1 || cfg.EnabledSkills[0] != "coding" {
		t.Errorf("EnabledSkills = %v, want [coding]", cfg.EnabledSkills)
	}
	if cfg.MaxTurns != 30 {
		t.Errorf("MaxTurns = %d, want 30", cfg.MaxTurns)
	}
	if cfg.Sandbox == nil {
		t.Fatal("Sandbox is nil")
	}
	if cfg.Sandbox.Root != "/workspace" {
		t.Errorf("Sandbox.Root = %q, want %q", cfg.Sandbox.Root, "/workspace")
	}
}

func TestLoadAgentConfig_DefaultNameFromFilename(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "my-agent.yaml")

	content := []byte(`model: claude-3`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig: %v", err)
	}

	if cfg.Name != "my-agent" {
		t.Errorf("Name = %q, want %q", cfg.Name, "my-agent")
	}
}

func TestLoadAgentConfig_DefaultMaxTurns(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bot.yaml")

	content := []byte(`model: gpt-4`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig: %v", err)
	}

	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", cfg.MaxTurns)
	}
}

func TestLoadAgentConfigs(t *testing.T) {
	dir := t.TempDir()

	writeConfig := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	writeConfig("alpha.yaml", `model: gpt-4
system_prompt: "Alpha agent"
`)
	writeConfig("beta.yaml", `model: claude-3
system_prompt: "Beta agent"
max_turns: 100
`)
	writeConfig("readme.txt", `this is not yaml`)

	configs, err := LoadAgentConfigs(dir)
	if err != nil {
		t.Fatalf("LoadAgentConfigs: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("len(configs) = %d, want 2", len(configs))
	}

	alpha, ok := configs["alpha"]
	if !ok {
		t.Fatal("missing alpha config")
	}
	if alpha.Model != "gpt-4" {
		t.Errorf("alpha.Model = %q, want %q", alpha.Model, "gpt-4")
	}

	beta, ok := configs["beta"]
	if !ok {
		t.Fatal("missing beta config")
	}
	if beta.MaxTurns != 100 {
		t.Errorf("beta.MaxTurns = %d, want 100", beta.MaxTurns)
	}
}

func TestLoadAgentConfigs_NonExistentDir(t *testing.T) {
	configs, err := LoadAgentConfigs("/no/such/directory")
	if err != nil {
		t.Fatalf("LoadAgentConfigs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("len(configs) = %d, want 0", len(configs))
	}
}

func TestLoadAgentConfigs_SkipNonYAML(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# notes"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	configs, err := LoadAgentConfigs(dir)
	if err != nil {
		t.Fatalf("LoadAgentConfigs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("len(configs) = %d, want 0", len(configs))
	}
}
