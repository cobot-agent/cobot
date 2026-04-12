package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cobot-agent/cobot/internal/xdg"
)

func Discover(startDir string) (*Workspace, error) {
	dir := startDir
	for {
		cobotDir := filepath.Join(dir, ".cobot")
		info, err := os.Stat(cobotDir)
		if err == nil && info.IsDir() {
			projectName := filepath.Base(dir)
			ws, err := LoadWorkspace(filepath.Join(
				xdg.ConfigDir(),
				"workspaces",
				projectName,
				"workspace.yaml",
			))
			if err == nil {
				return ws, nil
			}

			m, err := NewManager()
			if err != nil {
				return nil, fmt.Errorf("create manager: %w", err)
			}
			return m.CreateProject(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no .cobot directory found from %s", startDir)
		}
		dir = parent
	}
}

func DiscoverOrDefault(startDir string) (*Workspace, error) {
	ws, err := Discover(startDir)
	if err == nil {
		return ws, nil
	}
	return EnsureDefaultWorkspace()
}

func DiscoverOrCurrent(startDir string, manager *Manager) (*Workspace, error) {
	ws, err := Discover(startDir)
	if err == nil {
		return ws, nil
	}
	if manager != nil {
		return manager.Current(), nil
	}
	return EnsureDefaultWorkspace()
}
