package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatYAML      Format = "yaml"
	FormatMarkdown  Format = "markdown"
	FormatDirectory Format = "directory"
)

type Skill struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Trigger     string `yaml:"trigger" json:"trigger"`
	Format      Format `yaml:"-" json:"format"`
	Steps       []Step `yaml:"steps,omitempty" json:"steps,omitempty"`
	Content     string `yaml:"-" json:"content,omitempty"`
	Dir         string `yaml:"-" json:"-"`
}

type Step struct {
	Prompt string         `yaml:"prompt"`
	Tool   string         `yaml:"tool,omitempty"`
	Args   map[string]any `yaml:"args,omitempty"`
	Output string         `yaml:"output,omitempty"`
}

func LoadFile(path string) (*Skill, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return loadYAML(path)
	case ".md":
		return loadMarkdown(path)
	default:
		return nil, fmt.Errorf("unsupported skill file format: %s", ext)
	}
}

func loadYAML(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}
	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("parse skill %s: %w", path, err)
	}
	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	skill.Format = FormatYAML
	return &skill, nil
}

func loadMarkdown(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}

	content := string(data)
	var skill Skill

	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end >= 0 {
			frontmatter := content[3 : end+3]
			body := strings.TrimSpace(content[end+6:])

			if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
				return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
			}
			skill.Content = body
		}
	}

	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	skill.Format = FormatMarkdown
	return &skill, nil
}

func LoadDir(dir string) (*Skill, error) {
	skillPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md in %s: %w", dir, err)
	}

	content := string(data)
	var skill Skill

	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end >= 0 {
			frontmatter := content[3 : end+3]
			body := strings.TrimSpace(content[end+6:])

			if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
				return nil, fmt.Errorf("parse frontmatter in %s: %w", skillPath, err)
			}
			skill.Content = body
		}
	} else {
		skill.Content = strings.TrimSpace(content)
	}

	if skill.Name == "" {
		skill.Name = filepath.Base(dir)
	}
	skill.Format = FormatDirectory
	skill.Dir = dir

	scriptsDir := filepath.Join(dir, "scripts")
	if info, err := os.Stat(scriptsDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(scriptsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					skill.Steps = append(skill.Steps, Step{
						Tool:   "script",
						Args:   map[string]any{"file": entry.Name()},
						Output: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
					})
				}
			}
		}
	}

	return &skill, nil
}

func LoadRegistry(dir string) (map[string]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	registry := make(map[string]*Skill)
	for _, entry := range entries {
		var skill *Skill
		var loadErr error

		if entry.IsDir() {
			skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillPath); err == nil {
				skill, loadErr = LoadDir(filepath.Join(dir, entry.Name()))
			} else {
				continue
			}
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" && ext != ".md" {
				continue
			}
			skill, loadErr = LoadFile(filepath.Join(dir, entry.Name()))
		}

		if loadErr != nil {
			return nil, loadErr
		}
		if skill != nil {
			registry[skill.Name] = skill
		}
	}
	return registry, nil
}

func LoadByName(registryDir string, name string) (*Skill, error) {
	candidates := []string{
		filepath.Join(registryDir, name+".yaml"),
		filepath.Join(registryDir, name+".yml"),
		filepath.Join(registryDir, name+".md"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return LoadFile(c)
		}
	}

	dirCandidate := filepath.Join(registryDir, name)
	if info, err := os.Stat(dirCandidate); err == nil && info.IsDir() {
		skillPath := filepath.Join(dirCandidate, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			return LoadDir(dirCandidate)
		}
	}

	return nil, fmt.Errorf("skill %q not found in %s", name, registryDir)
}

func LoadFromYAML(path string) (*Skill, error) {
	return loadYAML(path)
}

func LoadFromDirLegacy(dir string) ([]*Skill, error) {
	registry, err := LoadRegistry(dir)
	if err != nil {
		return nil, err
	}
	var result []*Skill
	for _, s := range registry {
		result = append(result, s)
	}
	return result, nil
}
