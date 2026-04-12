package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type RegistryEntry struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Transport   string            `yaml:"transport"`
	Command     string            `yaml:"command,omitempty"`
	Args        []string          `yaml:"args,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	URL         string            `yaml:"url,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnv(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		return os.Getenv(varName)
	})
}

func LoadRegistry(dir string) (map[string]*RegistryEntry, error) {
	registry := make(map[string]*RegistryEntry)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, fmt.Errorf("read registry directory %q: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read registry file %q: %w", entry.Name(), err)
		}

		var regEntry RegistryEntry
		if err := yaml.Unmarshal(data, &regEntry); err != nil {
			return nil, fmt.Errorf("parse registry file %q: %w", entry.Name(), err)
		}

		if regEntry.Name == "" {
			regEntry.Name = strings.TrimSuffix(entry.Name(), ".yaml")
		}

		for k, v := range regEntry.Env {
			regEntry.Env[k] = expandEnv(v)
		}
		for k, v := range regEntry.Headers {
			regEntry.Headers[k] = expandEnv(v)
		}

		registry[regEntry.Name] = &regEntry
	}

	return registry, nil
}
