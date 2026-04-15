package main

import (
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestApplyChatFlags_OverridesModelAndPrompt(t *testing.T) {
	cfg := cobot.DefaultConfig()
	// Ensure defaults are set
	if cfg.Model == "" {
		t.Fatalf("default model should not be empty")
	}

	ApplyChatFlags(cfg, "openai:gpt-3.5-turbo", "Custom system prompt")
	if cfg.Model != "openai:gpt-3.5-turbo" {
		t.Fatalf("expected model override to be applied, got %s", cfg.Model)
	}
	if cfg.SystemPrompt != "Custom system prompt" {
		t.Fatalf("expected system prompt override to be applied, got %q", cfg.SystemPrompt)
	}
}

func TestApplyChatFlags_NoOverrideWhenEmpty(t *testing.T) {
	cfg := cobot.DefaultConfig()
	originalModel := cfg.Model
	originalPrompt := cfg.SystemPrompt

	ApplyChatFlags(cfg, "", "")
	if cfg.Model != originalModel {
		t.Fatalf("expected model to remain %q, got %q", originalModel, cfg.Model)
	}
	if cfg.SystemPrompt != originalPrompt {
		t.Fatalf("expected system prompt to remain %q, got %q", originalPrompt, cfg.SystemPrompt)
	}
}
