package cobot

import (
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ConfigPath   string                    `yaml:"config_path,omitempty"`
	DataPath     string                    `yaml:"data_path,omitempty"`
	Workspace    string                    `yaml:"workspace,omitempty"`
	Model        string                    `yaml:"model"`
	Temperature  float64                   `yaml:"temperature,omitempty"`
	MaxTurns     int                       `yaml:"max_turns"`
	SystemPrompt string                    `yaml:"system_prompt,omitempty"`
	Verbose      bool                      `yaml:"verbose,omitempty"`
	APIKeys      map[string]string         `yaml:"api_keys,omitempty"`
	Providers    map[string]ProviderConfig `yaml:"providers,omitempty"`
	Memory       MemoryConfig              `yaml:"memory,omitempty"`
}

type MemoryConfig struct {
	Enabled             bool          `yaml:"enabled"`
	IntelligentCuration bool          `yaml:"intelligent_curation"`
	CurationInterval    time.Duration `yaml:"curation_interval"`
	BadgerPath          string        `yaml:"badger_path,omitempty"`
	BlevePath           string        `yaml:"bleve_path,omitempty"`
}

type ProviderConfig struct {
	BaseURL string            `yaml:"base_url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type SandboxConfig struct {
	Root            string   `yaml:"root"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	ReadonlyPaths   []string `yaml:"readonly_paths,omitempty"`
	AllowNetwork    bool     `yaml:"allow_network"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`
}

func (s *SandboxConfig) IsAllowed(path string, write bool) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, p := range s.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return !write
		}
	}

	for _, p := range s.AllowPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return true
		}
	}

	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		if isSubpath(absPath, absRoot) {
			return true
		}
	}

	return false
}

func (s *SandboxConfig) IsBlockedCommand(cmd string) bool {
	for _, blocked := range s.BlockedCommands {
		if strings.Contains(cmd, blocked) {
			return true
		}
	}
	return false
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

func DefaultConfig() *Config {
	return &Config{
		MaxTurns: 50,
		Model:    "openai:gpt-4o",
		APIKeys:  make(map[string]string),
		Memory: MemoryConfig{
			Enabled:             true,
			IntelligentCuration: true,
			CurationInterval:    30 * time.Second,
		},
	}
}
