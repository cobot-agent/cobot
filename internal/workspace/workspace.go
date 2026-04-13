package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/cobot-agent/cobot/internal/xdg"
)

// WorkspaceType 定义 workspace 的类型
type WorkspaceType string

const (
	// WorkspaceTypeDefault 默认 workspace，始终存在
	WorkspaceTypeDefault WorkspaceType = "default"
	// WorkspaceTypeProject 与具体项目目录关联的 workspace
	WorkspaceTypeProject WorkspaceType = "project"
	// WorkspaceTypeCustom 用户自定义的 workspace
	WorkspaceTypeCustom WorkspaceType = "custom"
)

// Workspace 表示一个独立的工作空间
// 每个 workspace 拥有自己独立的配置、persona 文件和 memory 存储
type Workspace struct {
	// 元数据
	ID   string        `yaml:"id"`   // 唯一标识符
	Name string        `yaml:"name"` // 显示名称
	Type WorkspaceType `yaml:"type"` // workspace 类型

	// 路径配置
	Root       string `yaml:"root,omitempty"` // 项目目录（仅 project 类型）
	ConfigPath string `yaml:"config_path"`    // workspace.yaml 路径
	ConfigDir  string `yaml:"config_dir"`     // 配置目录（SOUL.md 等）
	DataDir    string `yaml:"data_dir"`       // 数据目录（memory 等）

	// 创建时间
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
}

// IsDefault 检查是否为默认 workspace
func (w *Workspace) IsDefault() bool {
	return w.Type == WorkspaceTypeDefault
}

// IsProject 检查是否为项目 workspace
func (w *Workspace) IsProject() bool {
	return w.Type == WorkspaceTypeProject
}

// GetSoulPath 返回 SOUL.md 路径
func (w *Workspace) GetSoulPath() string {
	return filepath.Join(w.ConfigDir, "SOUL.md")
}

// GetUserPath 返回 USER.md 路径
func (w *Workspace) GetUserPath() string {
	return filepath.Join(w.ConfigDir, "USER.md")
}

// GetMemoryMdPath 返回 MEMORY.md 路径
func (w *Workspace) GetMemoryMdPath() string {
	return filepath.Join(w.ConfigDir, "MEMORY.md")
}

// MemoryDir 返回 memory 数据目录
func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.DataDir, "memory")
}

// SessionsDir 返回 sessions 数据目录
func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.DataDir, "sessions")
}

// SkillsDir 返回 skills 数据目录
func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.DataDir, "skills")
}

// SchedulerDir 返回 scheduler 数据目录
func (w *Workspace) SchedulerDir() string {
	return filepath.Join(w.DataDir, "scheduler")
}

// AgentsMdPath 返回 AGENTS.md 路径（仅 project 类型）
func (w *Workspace) AgentsMdPath() string {
	if w.Root == "" {
		return ""
	}
	return filepath.Join(w.Root, ".cobot", "AGENTS.md")
}

// ToolsConfigPath 返回 tools.yaml 路径
func (w *Workspace) ToolsConfigPath() string {
	if w.Root != "" {
		return filepath.Join(w.Root, ".cobot", "tools.yaml")
	}
	return filepath.Join(w.ConfigDir, "tools.yaml")
}

// EnsureDirs 确保所有必要的目录都存在
func (w *Workspace) EnsureDirs() error {
	dirs := []string{
		w.ConfigDir,
		w.DataDir,
		w.MemoryDir(),
		w.SessionsDir(),
		w.SkillsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

// Save 保存 workspace 配置到文件
func (w *Workspace) Save() error {
	w.UpdatedAt = time.Now()
	data, err := yaml.Marshal(w)
	if err != nil {
		return fmt.Errorf("marshal workspace: %w", err)
	}
	if err := os.WriteFile(w.ConfigPath, data, 0644); err != nil {
		return fmt.Errorf("write workspace config: %w", err)
	}
	return nil
}

// LoadWorkspace 从配置文件加载 workspace
func LoadWorkspace(configPath string) (*Workspace, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read workspace config: %w", err)
	}
	var ws Workspace
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parse workspace config: %w", err)
	}
	return &ws, nil
}

// NewWorkspace 创建新的 workspace
func NewWorkspace(name string, wsType WorkspaceType, root string) *Workspace {
	now := time.Now()
	ws := &Workspace{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      wsType,
		Root:      root,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 设置路径
	workspacesDir := filepath.Join(xdg.ConfigDir(), "workspaces")
	workspaceDataDir := filepath.Join(xdg.DataDir(), "workspaces")

	switch wsType {
	case WorkspaceTypeDefault:
		ws.ConfigDir = filepath.Join(workspacesDir, "default")
		ws.DataDir = filepath.Join(workspaceDataDir, "default")
		ws.ConfigPath = filepath.Join(ws.ConfigDir, "workspace.yaml")
	case WorkspaceTypeProject:
		// 使用项目目录名作为 workspace 标识
		projectName := filepath.Base(root)
		ws.ConfigDir = filepath.Join(workspacesDir, projectName)
		ws.DataDir = filepath.Join(workspaceDataDir, projectName)
		ws.ConfigPath = filepath.Join(ws.ConfigDir, "workspace.yaml")
	case WorkspaceTypeCustom:
		ws.ConfigDir = filepath.Join(workspacesDir, name)
		ws.DataDir = filepath.Join(workspaceDataDir, name)
		ws.ConfigPath = filepath.Join(ws.ConfigDir, "workspace.yaml")
	}

	return ws
}

// DefaultWorkspace 返回默认 workspace
func DefaultWorkspace() *Workspace {
	ws := NewWorkspace("default", WorkspaceTypeDefault, "")
	// 如果 default workspace 已存在，加载它
	if _, err := os.Stat(ws.ConfigPath); err == nil {
		if loaded, err := LoadWorkspace(ws.ConfigPath); err == nil {
			return loaded
		}
	}
	return ws
}

// EnsureDefaultWorkspace 确保默认 workspace 存在
func EnsureDefaultWorkspace() (*Workspace, error) {
	ws := DefaultWorkspace()
	if err := ws.EnsureDirs(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(ws.ConfigPath); os.IsNotExist(err) {
		if err := ws.Save(); err != nil {
			return nil, err
		}
	}
	return ws, nil
}

// ValidatePathWithinWorkspace 验证路径是否在 workspace 范围内
// 这是权限控制的关键函数，确保 agent 只能访问当前 workspace 的文件
func (w *Workspace) ValidatePathWithinWorkspace(path string) error {
	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// 检查路径是否在 workspace 的数据目录内
	dataDir, err := filepath.Abs(w.DataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	if isSubpath(absPath, dataDir) {
		return nil
	}

	// 检查路径是否在 workspace 的配置目录内
	configDir, err := filepath.Abs(w.ConfigDir)
	if err != nil {
		return fmt.Errorf("resolve config dir: %w", err)
	}
	if isSubpath(absPath, configDir) {
		return nil
	}

	// 对于 project 类型的 workspace，还允许访问项目根目录
	if w.Root != "" {
		rootDir, err := filepath.Abs(w.Root)
		if err != nil {
			return fmt.Errorf("resolve root dir: %w", err)
		}
		if isSubpath(absPath, rootDir) {
			return nil
		}
	}

	return fmt.Errorf("path %s is outside of workspace boundaries", path)
}

// isSubpath 检查 path 是否是 base 的子路径
func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}
