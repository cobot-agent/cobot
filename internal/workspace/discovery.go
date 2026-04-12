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
			ws := &Workspace{
				Root:       dir,
				ConfigPath: filepath.Join(cobotDir, "config.yaml"),
				DataDir:    xdg.CobotDataDir(),
			}
			agentsMd := filepath.Join(cobotDir, "AGENTS.md")
			if _, err := os.Stat(agentsMd); err == nil {
				ws.AgentsMd = agentsMd
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

func DiscoverOrGlobal(startDir string) (*Workspace, error) {
	ws, err := Discover(startDir)
	if err == nil {
		return ws, nil
	}

	if err := EnsureGlobalWorkspace(); err != nil {
		return nil, fmt.Errorf("ensure global workspace: %w", err)
	}

	return GlobalWorkspace(), nil
}

func Init(dir string) (*Workspace, error) {
	cobotDir := filepath.Join(dir, ".cobot")
	if err := os.MkdirAll(cobotDir, 0755); err != nil {
		return nil, fmt.Errorf("create .cobot: %w", err)
	}

	configPath := filepath.Join(cobotDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := []byte("model: openai:gpt-4o\nmax_turns: 50\n")
		if err := os.WriteFile(configPath, defaultConfig, 0644); err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}
	}

	agentsMd := filepath.Join(cobotDir, "AGENTS.md")
	if _, err := os.Stat(agentsMd); os.IsNotExist(err) {
		if err := os.WriteFile(agentsMd, []byte("# Cobot Agent\n\nYou are a helpful AI assistant.\n"), 0644); err != nil {
			return nil, fmt.Errorf("write AGENTS.md: %w", err)
		}
	}

	if err := EnsureGlobalWorkspace(); err != nil {
		return nil, err
	}

	return &Workspace{
		Root:       dir,
		ConfigPath: configPath,
		AgentsMd:   agentsMd,
		DataDir:    xdg.CobotDataDir(),
	}, nil
}
