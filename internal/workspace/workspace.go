package workspace

import "path/filepath"

type Workspace struct {
	Root       string
	ConfigPath string
	AgentsMd   string
}

func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.Root, ".cobot", "memory")
}

func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.Root, ".cobot", "sessions")
}

func (w *Workspace) ToolsConfigPath() string {
	return filepath.Join(w.Root, ".cobot", "tools.yaml")
}

func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.Root, ".cobot", "skills")
}
