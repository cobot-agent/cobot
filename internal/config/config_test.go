package config

import (
	"os"
	"path/filepath"
	"testing"
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
