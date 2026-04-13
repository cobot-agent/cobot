package skills

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Trigger     string `yaml:"trigger"`
	Steps       []Step `yaml:"steps"`
}

type Step struct {
	Prompt string         `yaml:"prompt"`
	Tool   string         `yaml:"tool,omitempty"`
	Args   map[string]any `yaml:"args,omitempty"`
	Output string         `yaml:"output,omitempty"`
}

func LoadFromYAML(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}
	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("parse skill %s: %w", path, err)
	}
	if skill.Name == "" {
		skill.Name = filepath.Base(path)
	}
	return &skill, nil
}

func LoadFromDir(dir string) ([]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var skills []*Skill
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		skill, err := LoadFromYAML(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}
