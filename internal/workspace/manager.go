package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cobot-agent/cobot/internal/debug"
	"github.com/cobot-agent/cobot/internal/xdg"
)

type Manager struct {
	definitionsDir string
	dataDir        string
}

func NewManager() (*Manager, error) {
	m := &Manager{
		definitionsDir: xdg.WorkspaceDefinitionsDir(),
		dataDir:        xdg.DataDir(),
	}

	if err := os.MkdirAll(m.definitionsDir, 0755); err != nil {
		return nil, fmt.Errorf("create definitions dir: %w", err)
	}
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	if err := m.ensureDefault(); err != nil {
		return nil, fmt.Errorf("ensure default workspace: %w", err)
	}

	return m, nil
}

func (m *Manager) ensureDefault() error {
	defPath := filepath.Join(m.definitionsDir, "default.yaml")
	if _, err := os.Stat(defPath); os.IsNotExist(err) {
		def := &WorkspaceDefinition{
			Name: "default",
			Type: WorkspaceTypeDefault,
		}
		if err := saveDefinition(def, defPath); err != nil {
			return err
		}
	}

	ws, err := m.Resolve("default")
	if err != nil {
		return err
	}
	if err := ws.EnsureDirs(); err != nil {
		return err
	}
	if _, err := os.Stat(ws.ConfigPath()); os.IsNotExist(err) {
		if err := ws.SaveConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Resolve(name string) (*Workspace, error) {
	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	def, err := loadDefinition(defPath)
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %s", name)
	}
	ws := newWorkspaceFromDefinition(def, m.dataDir)
	return ws, nil
}

func (m *Manager) Create(name string, wsType WorkspaceType, root string, customPath string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}
	if name == "default" && wsType != WorkspaceTypeDefault {
		return nil, fmt.Errorf("name 'default' is reserved")
	}

	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	if _, err := os.Stat(defPath); err == nil {
		return nil, fmt.Errorf("workspace '%s' already exists", name)
	}

	def := &WorkspaceDefinition{
		Name: name,
		Type: wsType,
		Path: customPath,
		Root: root,
	}

	if err := saveDefinition(def, defPath); err != nil {
		return nil, err
	}

	ws := newWorkspaceFromDefinition(def, m.dataDir)
	ws.Config = newWorkspaceConfig(name, wsType, root)

	if err := ws.EnsureDirs(); err != nil {
		return nil, err
	}
	if err := ws.SaveConfig(); err != nil {
		return nil, err
	}

	return ws, nil
}

func (m *Manager) List() ([]*WorkspaceDefinition, error) {
	var defs []*WorkspaceDefinition

	entries, err := os.ReadDir(m.definitionsDir)
	if err != nil {
		return nil, fmt.Errorf("read definitions dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		defPath := filepath.Join(m.definitionsDir, entry.Name())
		def, err := loadDefinition(defPath)
		if err != nil {
			debug.Log("workspace", "skipping invalid definition", "file", entry.Name(), "error", err)
			continue
		}
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Type == WorkspaceTypeDefault {
			return true
		}
		if defs[j].Type == WorkspaceTypeDefault {
			return false
		}
		return defs[i].Name < defs[j].Name
	})

	return defs, nil
}

func (m *Manager) Delete(name string) error {
	if name == "default" {
		return fmt.Errorf("cannot delete default workspace")
	}

	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	def, err := loadDefinition(defPath)
	if err != nil {
		return fmt.Errorf("workspace not found: %s", name)
	}

	if err := os.Remove(defPath); err != nil {
		return fmt.Errorf("remove definition: %w", err)
	}

	dataPath := def.ResolvePath(m.dataDir)
	if err := os.RemoveAll(dataPath); err != nil {
		return fmt.Errorf("remove workspace data: %w", err)
	}

	return nil
}

func (m *Manager) Discover(startDir string) (*Workspace, error) {
	dir := startDir
	for {
		cobotDir := filepath.Join(dir, ".cobot")
		info, err := os.Stat(cobotDir)
		if err == nil && info.IsDir() {
			projectName := filepath.Base(dir)

			defPath := filepath.Join(m.definitionsDir, projectName+".yaml")
			if def, err := loadDefinition(defPath); err == nil {
				return newWorkspaceFromDefinition(def, m.dataDir), nil
			}

			return m.Create(projectName, WorkspaceTypeProject, dir, "")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no .cobot directory found from %s", startDir)
		}
		dir = parent
	}
}

func (m *Manager) ResolveByNameOrDiscover(name string, startDir string) (*Workspace, error) {
	if name != "" {
		ws, err := m.Resolve(name)
		if err == nil {
			return ws, nil
		}
	}

	ws, err := m.Discover(startDir)
	if err == nil {
		return ws, nil
	}

	return m.Resolve("default")
}
