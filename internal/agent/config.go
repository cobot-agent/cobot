package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cobot-agent/cobot/internal/util"
)

type AgentConfig struct {
	Name          string              `yaml:"name"`
	Model         string              `yaml:"model"`
	SystemPrompt  string              `yaml:"system_prompt"`
	EnabledMCP    []string            `yaml:"enabled_mcp,omitempty"`
	EnabledSkills []string            `yaml:"enabled_skills,omitempty"`
	MaxTurns      int                 `yaml:"max_turns,omitempty"`
	Sandbox       *AgentSandboxConfig `yaml:"sandbox,omitempty"`
}

type AgentSandboxConfig struct {
	Root            string   `yaml:"root,omitempty"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`
}

func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config %s: %w", path, err)
	}

	cfg := &AgentConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse agent config %s: %w", path, err)
	}

	if cfg.Name == "" {
		base := filepath.Base(path)
		cfg.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = util.DefaultMaxTurns
	}

	return cfg, nil
}

func LoadAgentConfigs(dir string) (map[string]*AgentConfig, error) {
	configs := make(map[string]*AgentConfig)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return configs, nil
		}
		return nil, fmt.Errorf("read agent configs dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		cfg, err := LoadAgentConfig(path)
		if err != nil {
			return nil, err
		}

		configs[cfg.Name] = cfg
	}

	return configs, nil
}
