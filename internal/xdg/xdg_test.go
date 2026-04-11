package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigHomeDefault(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config")
	if ConfigHome() != expected {
		t.Errorf("expected %s, got %s", expected, ConfigHome())
	}
}

func TestConfigHomeFromEnv(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")
	if ConfigHome() != dir {
		t.Errorf("expected %s, got %s", dir, ConfigHome())
	}
}

func TestDataHomeDefault(t *testing.T) {
	os.Unsetenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share")
	if DataHome() != expected {
		t.Errorf("expected %s, got %s", expected, DataHome())
	}
}

func TestDataHomeFromEnv(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", dir)
	defer os.Unsetenv("XDG_DATA_HOME")
	if DataHome() != dir {
		t.Errorf("expected %s, got %s", dir, DataHome())
	}
}

func TestCobotConfigDir(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")
	expected := filepath.Join(dir, "cobot")
	if CobotConfigDir() != expected {
		t.Errorf("expected %s, got %s", expected, CobotConfigDir())
	}
}

func TestCobotDataDir(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", dir)
	defer os.Unsetenv("XDG_DATA_HOME")
	expected := filepath.Join(dir, "cobot")
	if CobotDataDir() != expected {
		t.Errorf("expected %s, got %s", expected, CobotDataDir())
	}
}
