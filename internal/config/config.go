package config

import (
	"fmt"
	"os"
	"path/filepath"

	cobot "github.com/cobot-agent/cobot/pkg"
	"gopkg.in/yaml.v3"
)

func LoadFromFile(cfg *cobot.Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	expanded := ExpandEnvVars(string(data))
	return yaml.Unmarshal([]byte(expanded), cfg)
}

func ApplyEnvVars(cfg *cobot.Config) {
	if v := os.Getenv("COBOT_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("COBOT_WORKSPACE"); v != "" {
		cfg.Workspace = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.EnsureAPIKeys()
		cfg.APIKeys["openai"] = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.EnsureAPIKeys()
		cfg.APIKeys["anthropic"] = v
	}
}

func LoadWorkspaceConfig(cfg *cobot.Config, workspaceDir string) error {
	wsConfig := filepath.Join(workspaceDir, ".cobot", "config.yaml")
	if _, err := os.Stat(wsConfig); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat workspace config: %w", err)
	}
	return LoadFromFile(cfg, wsConfig)
}

func SaveToFile(cfg *cobot.Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return SaveYAML(path, cfg)
}
