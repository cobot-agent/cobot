package skills

import (
	"bytes"
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

func LoadFromYAML(path string) (*Skill, error) {
	skill, err := loadYAML(path)
	if err != nil {
		return nil, err
	}
	skill.Format = FormatYAML
	return skill, nil
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
	return &skill, nil
}

func loadMarkdown(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("parse skill %s: missing frontmatter", path)
	}

	end := bytes.Index([]byte(content[3:]), []byte("\n---"))
	if end < 0 {
		return nil, fmt.Errorf("parse skill %s: unclosed frontmatter", path)
	}

	frontmatter := content[3 : end+3]
	body := strings.TrimSpace(content[end+3+4:])

	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("parse skill %s frontmatter: %w", path, err)
	}
	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	skill.Content = body
	skill.Format = FormatMarkdown
	return &skill, nil
}

func LoadFile(path string) (*Skill, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return LoadFromYAML(path)
	case ".md":
		return loadMarkdown(path)
	default:
		return nil, fmt.Errorf("unsupported skill file extension: %s", ext)
	}
}

func LoadDir(dir string) (*Skill, error) {
	skillMD := filepath.Join(dir, "SKILL.md")
	info, err := os.Stat(skillMD)
	if err != nil {
		return nil, fmt.Errorf("skill directory %s: no SKILL.md found", dir)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("skill directory %s: SKILL.md is a directory", dir)
	}

	skill, err := loadMarkdown(skillMD)
	if err != nil {
		return nil, err
	}
	skill.Format = FormatDirectory
	skill.Dir = dir
	return skill, nil
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

func LoadRegistry(dir string) (map[string]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read registry dir %s: %w", dir, err)
	}

	registry := make(map[string]*Skill)
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)

		var skill *Skill
		var loadErr error

		if entry.IsDir() {
			skillMD := filepath.Join(path, "SKILL.md")
			if _, statErr := os.Stat(skillMD); statErr == nil {
				skill, loadErr = LoadDir(path)
			}
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".yaml" || ext == ".yml" || ext == ".md" {
				skill, loadErr = LoadFile(path)
			}
		}

		if loadErr != nil {
			return nil, fmt.Errorf("load skill %s: %w", name, loadErr)
		}
		if skill != nil {
			registry[skill.Name] = skill
		}
	}
	return registry, nil
}

func LoadByName(registryDir string, name string) (*Skill, error) {
	yamlPath := filepath.Join(registryDir, name+".yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return LoadFile(yamlPath)
	}
	ymlPath := filepath.Join(registryDir, name+".yml")
	if _, err := os.Stat(ymlPath); err == nil {
		return LoadFile(ymlPath)
	}
	mdPath := filepath.Join(registryDir, name+".md")
	if _, err := os.Stat(mdPath); err == nil {
		return LoadFile(mdPath)
	}
	dirPath := filepath.Join(registryDir, name)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		skillMD := filepath.Join(dirPath, "SKILL.md")
		if _, err := os.Stat(skillMD); err == nil {
			return LoadDir(dirPath)
		}
	}
	return nil, fmt.Errorf("skill %q not found in %s", name, registryDir)
}
