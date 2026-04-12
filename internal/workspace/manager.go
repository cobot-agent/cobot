package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cobot-agent/cobot/internal/debug"
	"github.com/cobot-agent/cobot/internal/xdg"
)

// Manager 管理多个 workspace
type Manager struct {
	workspacesDir     string
	workspacesDataDir string
	current           *Workspace
}

// ManagerConfig 存储 manager 的配置
type ManagerConfig struct {
	CurrentWorkspaceID string `yaml:"current_workspace_id"`
	Version            string `yaml:"version"`
}

// managerConfigPath 返回 manager 配置文件路径
func managerConfigPath() string {
	return filepath.Join(xdg.CobotConfigDir(), "workspaces", "manager.yaml")
}

// NewManager 创建新的 workspace manager
func NewManager() (*Manager, error) {
	m := &Manager{
		workspacesDir:     filepath.Join(xdg.CobotConfigDir(), "workspaces"),
		workspacesDataDir: filepath.Join(xdg.CobotDataDir(), "workspaces"),
	}

	// 确保目录存在
	if err := os.MkdirAll(m.workspacesDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspaces dir: %w", err)
	}
	if err := os.MkdirAll(m.workspacesDataDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspaces data dir: %w", err)
	}

	// 确保默认 workspace 存在
	if _, err := EnsureDefaultWorkspace(); err != nil {
		return nil, fmt.Errorf("ensure default workspace: %w", err)
	}

	// 加载当前 workspace
	if err := m.loadCurrent(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadCurrent 加载当前 workspace
func (m *Manager) loadCurrent() error {
	configPath := managerConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 首次使用，使用 default workspace
			m.current = DefaultWorkspace()
			return nil
		}
		return fmt.Errorf("read manager config: %w", err)
	}

	var cfg ManagerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse manager config: %w", err)
	}

	// 尝试加载指定的 workspace
	if cfg.CurrentWorkspaceID != "" {
		ws, err := m.GetByID(cfg.CurrentWorkspaceID)
		if err == nil {
			m.current = ws
			return nil
		}
	}

	// 回退到 default workspace
	m.current = DefaultWorkspace()
	return nil
}

// saveManagerConfig 保存 manager 配置
func (m *Manager) saveManagerConfig() error {
	cfg := ManagerConfig{
		CurrentWorkspaceID: m.current.ID,
		Version:            "1.0",
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal manager config: %w", err)
	}
	configPath := managerConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write manager config: %w", err)
	}
	return nil
}

// Create 创建新的 workspace
func (m *Manager) Create(name string, wsType WorkspaceType, root string) (*Workspace, error) {
	// 验证名称
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}
	if name == "default" && wsType != WorkspaceTypeDefault {
		return nil, fmt.Errorf("name 'default' is reserved")
	}

	// 检查是否已存在
	if _, err := m.GetByName(name); err == nil {
		return nil, fmt.Errorf("workspace '%s' already exists", name)
	}

	ws := NewWorkspace(name, wsType, root)

	// 确保目录存在
	if err := ws.EnsureDirs(); err != nil {
		return nil, err
	}

	// 保存配置
	if err := ws.Save(); err != nil {
		return nil, err
	}

	return ws, nil
}

// CreateProject 从项目目录创建 workspace
func (m *Manager) CreateProject(projectDir string) (*Workspace, error) {
	// 验证项目目录
	info, err := os.Stat(projectDir)
	if err != nil {
		return nil, fmt.Errorf("project directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", projectDir)
	}

	name := filepath.Base(projectDir)
	return m.Create(name, WorkspaceTypeProject, projectDir)
}

// List 列出所有 workspace
func (m *Manager) List() ([]*Workspace, error) {
	var workspaces []*Workspace

	// 确保 default workspace 存在
	defaultWs, err := EnsureDefaultWorkspace()
	if err != nil {
		return nil, err
	}
	workspaces = append(workspaces, defaultWs)

	// 读取 workspaces 目录
	entries, err := os.ReadDir(m.workspacesDir)
	if err != nil {
		return nil, fmt.Errorf("read workspaces dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == "default" {
			continue // 已经添加过了
		}

		configPath := filepath.Join(m.workspacesDir, entry.Name(), "workspace.yaml")
		ws, err := LoadWorkspace(configPath)
		if err != nil {
			debug.Log("workspace", "skipping invalid workspace", "dir", entry.Name(), "error", err)
			continue
		}
		workspaces = append(workspaces, ws)
	}

	// 按名称排序
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].IsDefault() {
			return true
		}
		if workspaces[j].IsDefault() {
			return false
		}
		return workspaces[i].Name < workspaces[j].Name
	})

	return workspaces, nil
}

// GetByID 通过 ID 获取 workspace
func (m *Manager) GetByID(id string) (*Workspace, error) {
	// 检查 default workspace
	defaultWs := DefaultWorkspace()
	if defaultWs.ID == id {
		// 重新加载以获取最新状态
		if loaded, err := LoadWorkspace(defaultWs.ConfigPath); err == nil {
			return loaded, nil
		}
		return defaultWs, nil
	}

	// 搜索其他 workspace
	workspaces, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, ws := range workspaces {
		if ws.ID == id {
			return ws, nil
		}
	}

	return nil, fmt.Errorf("workspace not found: %s", id)
}

// GetByName 通过名称获取 workspace
func (m *Manager) GetByName(name string) (*Workspace, error) {
	if name == "default" {
		return EnsureDefaultWorkspace()
	}

	configPath := filepath.Join(m.workspacesDir, name, "workspace.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace not found: %s", name)
	}

	return LoadWorkspace(configPath)
}

// Current 返回当前 workspace
func (m *Manager) Current() *Workspace {
	return m.current
}

// Switch 切换到指定的 workspace
func (m *Manager) Switch(idOrName string) (*Workspace, error) {
	var ws *Workspace
	var err error

	// 先尝试通过名称查找
	ws, err = m.GetByName(idOrName)
	if err != nil {
		// 再尝试通过 ID 查找
		ws, err = m.GetByID(idOrName)
		if err != nil {
			return nil, fmt.Errorf("workspace not found: %s", idOrName)
		}
	}

	m.current = ws
	if err := m.saveManagerConfig(); err != nil {
		return nil, err
	}

	return ws, nil
}

// Delete 删除 workspace
func (m *Manager) Delete(idOrName string) error {
	ws, err := m.GetByName(idOrName)
	if err != nil {
		ws, err = m.GetByID(idOrName)
		if err != nil {
			return err
		}
	}

	// 不能删除 default workspace
	if ws.IsDefault() {
		return fmt.Errorf("cannot delete default workspace")
	}

	// 不能删除当前正在使用的 workspace
	if m.current.ID == ws.ID {
		return fmt.Errorf("cannot delete current workspace, switch to another first")
	}

	// 删除配置目录
	if err := os.RemoveAll(ws.ConfigDir); err != nil {
		return fmt.Errorf("remove workspace config: %w", err)
	}

	// 删除数据目录
	if err := os.RemoveAll(ws.DataDir); err != nil {
		return fmt.Errorf("remove workspace data: %w", err)
	}

	return nil
}

// Rename 重命名 workspace
func (m *Manager) Rename(oldName, newName string) error {
	if oldName == "default" {
		return fmt.Errorf("cannot rename default workspace")
	}

	ws, err := m.GetByName(oldName)
	if err != nil {
		return err
	}

	// 检查新名称是否已存在
	if _, err := m.GetByName(newName); err == nil {
		return fmt.Errorf("workspace '%s' already exists", newName)
	}

	// 更新名称
	ws.Name = newName
	ws.UpdatedAt = time.Now()

	// 对于非 project 类型，需要移动目录
	if !ws.IsProject() {
		oldConfigDir := ws.ConfigDir
		oldDataDir := ws.DataDir

		ws.ConfigDir = filepath.Join(m.workspacesDir, newName)
		ws.DataDir = filepath.Join(m.workspacesDataDir, newName)
		ws.ConfigPath = filepath.Join(ws.ConfigDir, "workspace.yaml")

		// 移动配置目录
		if err := os.Rename(oldConfigDir, ws.ConfigDir); err != nil {
			return fmt.Errorf("rename config dir: %w", err)
		}

		// 移动数据目录
		if err := os.Rename(oldDataDir, ws.DataDir); err != nil {
			// 回滚配置目录
			if rollbackErr := os.Rename(ws.ConfigDir, oldConfigDir); rollbackErr != nil {
				return fmt.Errorf("rename data dir failed (%v) and rollback also failed (%v)", err, rollbackErr)
			}
			return fmt.Errorf("rename data dir: %w", err)
		}
	}

	// 保存更新后的配置
	if err := ws.Save(); err != nil {
		return err
	}

	// 如果重命名的是当前 workspace，更新 manager 配置
	if m.current.ID == ws.ID {
		m.current = ws
		if err := m.saveManagerConfig(); err != nil {
			return err
		}
	}

	return nil
}
