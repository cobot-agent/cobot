package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cobot-agent/cobot/internal/xdg"
)

type Manager struct {
	definitionsDir string
	dataDir        string
}

func NewManager() (*Manager, error) {
	defsDir := xdg.WorkspaceDefinitionsDir()
	dataDir := xdg.DataDir()

	if err := os.MkdirAll(defsDir, 0755); err != nil {
		return nil, fmt.Errorf("create definitions dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	m := &Manager{
		definitionsDir: defsDir,
		dataDir:        dataDir,
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
		return ws.SaveConfig()
	}
	return nil
}

func (m *Manager) Resolve(name string) (*Workspace, error) {
	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	def, err := loadDefinition(defPath)
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %s", name)
	}

	dataDir := def.ResolvePath(m.dataDir)
	cfgPath := filepath.Join(dataDir, "workspace.yaml")

	var cfg *WorkspaceConfig
	cfgData, err := loadWorkspaceConfig(cfgPath)
	if err != nil {
		cfg = newWorkspaceConfig(def.Name, def.Type, def.Root)
	} else {
		cfg = cfgData
	}

	return &Workspace{
		Definition: def,
		Config:     cfg,
		DataDir:    dataDir,
	}, nil
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
		Root: root,
	}
	if customPath != "" {
		def.Path = customPath
	}

	if err := saveDefinition(def, defPath); err != nil {
		return nil, err
	}

	dataDir := def.ResolvePath(m.dataDir)
	cfg := newWorkspaceConfig(name, wsType, root)

	ws := &Workspace{
		Definition: def,
		Config:     cfg,
		DataDir:    dataDir,
	}

	if err := ws.EnsureDirs(); err != nil {
		return nil, err
	}
	if err := ws.SaveConfig(); err != nil {
		return nil, err
	}

	return ws, nil
}

func (m *Manager) List() ([]*WorkspaceDefinition, error) {
	entries, err := os.ReadDir(m.definitionsDir)
	if err != nil {
		return nil, fmt.Errorf("read definitions dir: %w", err)
	}

	var defs []*WorkspaceDefinition
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		defPath := filepath.Join(m.definitionsDir, entry.Name())
		def, err := loadDefinition(defPath)
		if err != nil {
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

	dataDir := def.ResolvePath(m.dataDir)
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("remove workspace data: %w", err)
	}

	if err := os.Remove(defPath); err != nil {
		return fmt.Errorf("remove definition: %w", err)
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
			if _, err := os.Stat(defPath); err == nil {
				return m.Resolve(projectName)
			}
			ws, err := m.Create(projectName, WorkspaceTypeProject, dir, "")
			if err != nil {
				return nil, err
			}
			return ws, nil
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
		return m.Resolve(name)
	}
	ws, err := m.Discover(startDir)
	if err == nil {
		return ws, nil
	}
	return m.Resolve("default")
}
