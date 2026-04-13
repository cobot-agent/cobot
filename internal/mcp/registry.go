package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cobot-agent/cobot/internal/config"
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

func LoadRegistry(dir string) (map[string]*RegistryEntry, error) {
	entries := make(map[string]*RegistryEntry)

	infos, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, fmt.Errorf("read registry directory %q: %w", dir, err)
	}

	for _, info := range infos {
		if info.IsDir() || filepath.Ext(info.Name()) != ".yaml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, info.Name()))
		if err != nil {
			return nil, fmt.Errorf("read registry file %q: %w", info.Name(), err)
		}

		var entry RegistryEntry
		if err := yaml.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("parse registry file %q: %w", info.Name(), err)
		}

		if entry.Name == "" {
			entry.Name = strings.TrimSuffix(info.Name(), ".yaml")
		}

		for k, v := range entry.Env {
			entry.Env[k] = config.ExpandEnvVars(v)
		}
		for k, v := range entry.Headers {
			entry.Headers[k] = config.ExpandEnvVars(v)
		}

		entries[entry.Name] = &entry
	}

	return entries, nil
}
