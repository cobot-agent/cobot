package xdg

import (
	"os"
	"path/filepath"
)

func ConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, _ := UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func CobotConfigDir() string {
	return filepath.Join(ConfigHome(), "cobot")
}

func CobotDataDir() string {
	return filepath.Join(DataHome(), "cobot")
}

func GlobalConfigPath() string {
	return filepath.Join(CobotConfigDir(), "config.yaml")
}

func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
