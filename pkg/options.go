package cobot

import (
	"path/filepath"
	"strings"
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
	absPath = EvalSymlinks(absPath)

	readonlyMatched := false
	for _, rp := range s.ReadonlyPaths {
		absRP, err := filepath.Abs(rp)
		if err != nil {
			continue
		}
		absRP = EvalSymlinks(absRP)
		if IsSubpath(absPath, absRP) {
			readonlyMatched = true
			if write {
				return false
			}
		}
	}
	// If a readonly path matched and this is a read operation, allow access
	// even if the path is not under Root or AllowPaths.
	if readonlyMatched && !write {
		return true
	}

	for _, ap := range s.AllowPaths {
		absAP, err := filepath.Abs(ap)
		if err != nil {
			continue
		}
		absAP = EvalSymlinks(absAP)
		if IsSubpath(absPath, absAP) {
			return true
		}
	}

	if s.Root != "" {
		absRoot, err := filepath.Abs(s.Root)
		if err != nil {
			return false
		}
		absRoot = EvalSymlinks(absRoot)
		if IsSubpath(absPath, absRoot) {
			if readonlyMatched && write {
				return false
			}
			return true
		}
	}

	return false
}

func (s *SandboxConfig) IsBlockedCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	baseCmd := filepath.Base(fields[0])
	for _, blocked := range s.BlockedCommands {
		if baseCmd == blocked || cmd == blocked || strings.HasPrefix(cmd, blocked+" ") || strings.HasPrefix(cmd, blocked) {
			return true
		}
		if strings.Contains(cmd, "|"+blocked) || strings.Contains(cmd, ";"+blocked) ||
			strings.Contains(cmd, "$("+blocked) || strings.Contains(cmd, "`"+blocked) {
			return true
		}
	}
	return false
}

type MemoryConfig struct {
	Enabled bool   `yaml:"enabled"`
	DBPath  string `yaml:"db_path"`
}

type ProviderConfig struct {
	BaseURL string            `yaml:"base_url"`
	Headers map[string]string `yaml:"headers"`
}

func DefaultConfig() *Config {
	return &Config{
		MaxTurns: DefaultMaxTurns,
		Model:    "openai:gpt-4o",
		APIKeys:  make(map[string]string),
		Memory: MemoryConfig{
			Enabled: true,
		},
	}
}
