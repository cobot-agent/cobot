package config

import (
	"os"
	"path/filepath"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestLoadDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxTurns != 50 {
		t.Errorf("expected MaxTurns=50, got %d", cfg.MaxTurns)
	}
	if cfg.Model != "openai:gpt-4o" {
		t.Errorf("expected Model=openai:gpt-4o, got %s", cfg.Model)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("model: anthropic:claude-3.5-sonnet\nmax_turns: 30\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := LoadFromFile(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "anthropic:claude-3.5-sonnet" {
		t.Errorf("expected anthropic:claude-3.5-sonnet, got %s", cfg.Model)
	}
	if cfg.MaxTurns != 30 {
		t.Errorf("expected MaxTurns=30, got %d", cfg.MaxTurns)
	}
}

func TestFlagOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("model: anthropic:claude-3.5-sonnet\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := LoadFromFile(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	cfg.Model = "openai:gpt-4o-mini"
	if cfg.Model != "openai:gpt-4o-mini" {
		t.Errorf("expected flag override openai:gpt-4o-mini, got %s", cfg.Model)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	os.Setenv("TEST_API_KEY", "sk-test-123")
	defer os.Unsetenv("TEST_API_KEY")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("api_keys:\n  openai: ${TEST_API_KEY}\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := LoadFromFile(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	if cfg.APIKeys["openai"] != "sk-test-123" {
		t.Errorf("expected expanded env var, got %s", cfg.APIKeys["openai"])
	}
}

func TestApplyEnvVars(t *testing.T) {
	t.Setenv("COBOT_MODEL", "test-model")
	t.Setenv("OPENAI_API_KEY", "sk-test123")

	cfg := cobot.DefaultConfig()
	ApplyEnvVars(cfg)

	if cfg.Model != "test-model" {
		t.Errorf("expected test-model, got %s", cfg.Model)
	}
	if cfg.APIKeys["openai"] != "sk-test123" {
		t.Error("expected openai API key")
	}
}

func TestLoadWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, ".cobot")
	os.MkdirAll(wsDir, 0755)

	yamlData := "model: workspace-model\nmax_turns: 99\n"
	os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte(yamlData), 0644)

	cfg := cobot.DefaultConfig()
	err := LoadWorkspaceConfig(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "workspace-model" {
		t.Errorf("expected workspace-model, got %s", cfg.Model)
	}
	if cfg.MaxTurns != 99 {
		t.Errorf("expected 99, got %d", cfg.MaxTurns)
	}
}

func TestLoadWorkspaceConfigMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := cobot.DefaultConfig()
	err := LoadWorkspaceConfig(cfg, dir)
	if err != nil {
		t.Fatal("expected nil error for missing workspace config")
	}
}
