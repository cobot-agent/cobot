package xdg

import (
	"os"
	"path/filepath"
)

func configHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func dataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func ConfigDir() string {
	if v := os.Getenv("COBOT_CONFIG_PATH"); v != "" {
		return v
	}
	return filepath.Join(configHome(), "cobot")
}

func DataDir() string {
	if v := os.Getenv("COBOT_DATA_PATH"); v != "" {
		return v
	}
	return filepath.Join(dataHome(), "cobot")
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
