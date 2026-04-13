package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
	"gopkg.in/yaml.v3"
)

var envVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

func LoadFromFile(cfg *cobot.Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	expanded := expandEnvVars(string(data))
	return yaml.Unmarshal([]byte(expanded), cfg)
}

func expandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "${}")
		return os.Getenv(varName)
	})
}

func ApplyEnvVars(cfg *cobot.Config) {
	if v := os.Getenv("COBOT_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("COBOT_WORKSPACE"); v != "" {
		cfg.Workspace = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		if cfg.APIKeys == nil {
			cfg.APIKeys = make(map[string]string)
		}
		cfg.APIKeys["openai"] = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		if cfg.APIKeys == nil {
			cfg.APIKeys = make(map[string]string)
		}
		cfg.APIKeys["anthropic"] = v
	}
}

func LoadWorkspaceConfig(cfg *cobot.Config, workspaceDir string) error {
	wsConfig := filepath.Join(workspaceDir, ".cobot", "config.yaml")
	if _, err := os.Stat(wsConfig); err != nil {
		return nil
	}
	return LoadFromFile(cfg, wsConfig)
}

func SaveToFile(cfg *cobot.Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
