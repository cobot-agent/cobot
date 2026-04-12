package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cobot-agent/cobot/internal/xdg"
)

type Workspace struct {
	Root       string
	ConfigPath string
	AgentsMd   string
	DataDir    string
}

func (w *Workspace) MemoryDir() string       { return filepath.Join(w.DataDir, "memory") }
func (w *Workspace) SessionsDir() string     { return filepath.Join(w.DataDir, "sessions") }
func (w *Workspace) SkillsDir() string       { return filepath.Join(w.DataDir, "skills") }
func (w *Workspace) SchedulerDir() string    { return filepath.Join(w.DataDir, "scheduler") }
func (w *Workspace) ToolsConfigPath() string { return filepath.Join(w.Root, ".cobot", "tools.yaml") }

func GlobalWorkspace() *Workspace {
	return &Workspace{
		Root:       xdg.CobotConfigDir(),
		ConfigPath: xdg.GlobalConfigPath(),
		DataDir:    xdg.CobotDataDir(),
	}
}

func GlobalMemoryDir() string    { return filepath.Join(xdg.CobotDataDir(), "memory") }
func GlobalSessionsDir() string  { return filepath.Join(xdg.CobotDataDir(), "sessions") }
func GlobalSkillsDir() string    { return filepath.Join(xdg.CobotDataDir(), "skills") }
func GlobalSchedulerDir() string { return filepath.Join(xdg.CobotDataDir(), "scheduler") }
func GlobalSoulPath() string     { return filepath.Join(xdg.CobotConfigDir(), "SOUL.md") }
func GlobalUserPath() string     { return filepath.Join(xdg.CobotConfigDir(), "USER.md") }
func GlobalMemoryMdPath() string { return filepath.Join(xdg.CobotConfigDir(), "MEMORY.md") }
func GlobalToolsPath() string    { return filepath.Join(xdg.CobotConfigDir(), "tools.yaml") }

func EnsureGlobalWorkspace() error {
	dirs := []string{
		xdg.CobotConfigDir(),
		xdg.CobotDataDir(),
		GlobalMemoryDir(),
		GlobalSessionsDir(),
		GlobalSkillsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}
