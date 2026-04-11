package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/cobot-agent/cobot/internal/xdg"
)

type Workspace struct {
	Root       string
	ConfigPath string
	AgentsMd   string
	DataDir    string
}

func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.DataDir, "memory")
}

func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.DataDir, "sessions")
}

func (w *Workspace) ToolsConfigPath() string {
	return filepath.Join(w.Root, ".cobot", "tools.yaml")
}

func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.DataDir, "skills")
}

func (w *Workspace) SchedulerDir() string {
	return filepath.Join(w.DataDir, "scheduler")
}

func workspaceDataDir(projectRoot string) string {
	hash := sha256.Sum256([]byte(projectRoot))
	short := hex.EncodeToString(hash[:])[:16]
	return filepath.Join(xdg.CobotDataDir(), "workspaces", short)
}
