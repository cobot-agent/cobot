package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type WorkspaceType string

const (
	WorkspaceTypeDefault WorkspaceType = "default"
	WorkspaceTypeProject WorkspaceType = "project"
	WorkspaceTypeCustom  WorkspaceType = "custom"
)

type WorkspaceDefinition struct {
	Name string        `yaml:"name"`
	Type WorkspaceType `yaml:"type"`
	Path string        `yaml:"path,omitempty"`
	Root string        `yaml:"root,omitempty"`
}

func (d *WorkspaceDefinition) ResolvePath(dataDir string) string {
	if d.Path != "" {
		return d.Path
	}
	return filepath.Join(dataDir, d.Name)
}

type WorkspaceConfig struct {
	ID            string               `yaml:"id"`
	Name          string               `yaml:"name"`
	Type          WorkspaceType        `yaml:"type"`
	Root          string               `yaml:"root,omitempty"`
	CreatedAt     time.Time            `yaml:"created_at"`
	UpdatedAt     time.Time            `yaml:"updated_at"`
	EnabledMCP    []string             `yaml:"enabled_mcp,omitempty"`
	EnabledSkills []string             `yaml:"enabled_skills,omitempty"`
	Sandbox       SandboxConfig        `yaml:"sandbox,omitempty"`
	Agents        map[string]string    `yaml:"agents,omitempty"`
	DefaultAgent  string               `yaml:"default_agent,omitempty"`
	Summarization *SummarizationConfig `yaml:"summarization,omitempty"`
}

type SandboxConfig struct {
	Root            string   `yaml:"root,omitempty"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	ReadonlyPaths   []string `yaml:"readonly_paths,omitempty"`
	AllowNetwork    bool     `yaml:"allow_network"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`
}

type SummarizationConfig struct {
	Enabled bool     `yaml:"enabled"`
	Include []string `yaml:"include,omitempty"`
}

type Workspace struct {
	Definition *WorkspaceDefinition
	Config     *WorkspaceConfig
	DataDir    string
}

func (w *Workspace) IsDefault() bool {
	return w.Definition.Type == WorkspaceTypeDefault
}

func (w *Workspace) IsProject() bool {
	return w.Definition.Type == WorkspaceTypeProject
}

func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.DataDir, "memory")
}

func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.DataDir, "sessions")
}

func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.DataDir, "skills")
}

func (w *Workspace) SchedulerDir() string {
	return filepath.Join(w.DataDir, "scheduler")
}

func (w *Workspace) AgentsDir() string {
	return filepath.Join(w.DataDir, "agents")
}

func (w *Workspace) GetSoulPath() string {
	return filepath.Join(w.DataDir, "SOUL.md")
}

func (w *Workspace) GetUserPath() string {
	return filepath.Join(w.DataDir, "USER.md")
}

func (w *Workspace) GetMemoryMdPath() string {
	return filepath.Join(w.DataDir, "MEMORY.md")
}

func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.DataDir, "workspace.yaml")
}

func (w *Workspace) AgentsMdPath() string {
	if w.Definition.Root == "" {
		return ""
	}
	return filepath.Join(w.Definition.Root, ".cobot", "AGENTS.md")
}

func (w *Workspace) EnsureDirs() error {
	dirs := []string{
		w.DataDir,
		w.MemoryDir(),
		w.SessionsDir(),
		w.SkillsDir(),
		w.AgentsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (w *Workspace) SaveConfig() error {
	w.Config.UpdatedAt = time.Now()
	data, err := yaml.Marshal(w.Config)
	if err != nil {
		return fmt.Errorf("marshal workspace config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(w.ConfigPath()), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(w.ConfigPath(), data, 0644); err != nil {
		return fmt.Errorf("write workspace config: %w", err)
	}
	return nil
}

func (w *Workspace) ValidatePath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	dataDir, err := filepath.Abs(w.DataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	if isSubpath(absPath, dataDir) {
		return nil
	}

	if w.Definition.Root != "" {
		rootDir, err := filepath.Abs(w.Definition.Root)
		if err != nil {
			return fmt.Errorf("resolve root dir: %w", err)
		}
		if isSubpath(absPath, rootDir) {
			return nil
		}
	}

	return fmt.Errorf("path %s is outside of workspace boundaries", path)
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func saveDefinition(d *WorkspaceDefinition, path string) error {
	data, err := yaml.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal definition: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func loadDefinition(path string) (*WorkspaceDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read definition: %w", err)
	}
	var d WorkspaceDefinition
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse definition: %w", err)
	}
	return &d, nil
}

func loadWorkspaceConfig(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workspace config: %w", err)
	}
	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse workspace config: %w", err)
	}
	return &cfg, nil
}

func newWorkspaceConfig(name string, wsType WorkspaceType, root string) *WorkspaceConfig {
	now := time.Now()
	return &WorkspaceConfig{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      wsType,
		Root:      root,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
