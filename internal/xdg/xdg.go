package xdg

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	if v := os.Getenv("COBOT_CONFIG_PATH"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "cobot")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cobot")
}

func DataDir() string {
	if v := os.Getenv("COBOT_DATA_PATH"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "cobot")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "cobot")
}

func GlobalConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func MCPRegistryDir() string {
	return filepath.Join(ConfigDir(), "mcp")
}

func SkillsRegistryDir() string {
	return filepath.Join(ConfigDir(), "skills")
}

func WorkspaceDefinitionsDir() string {
	return filepath.Join(ConfigDir(), "workspaces")
}

func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
