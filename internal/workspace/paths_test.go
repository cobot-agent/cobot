package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirDefault(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Unsetenv("XDG_CONFIG_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "cobot")
	if got := ConfigDir(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestConfigDirFromCobotEnv(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("COBOT_CONFIG_PATH", dir)
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	if got := ConfigDir(); got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestConfigDirFromXDG(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")
	expected := filepath.Join(dir, "cobot")
	if got := ConfigDir(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestConfigDirCobotOverridesXDG(t *testing.T) {
	cobotDir := t.TempDir()
	xdgDir := t.TempDir()
	os.Setenv("COBOT_CONFIG_PATH", cobotDir)
	os.Setenv("XDG_CONFIG_HOME", xdgDir)
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	defer os.Unsetenv("XDG_CONFIG_HOME")
	if got := ConfigDir(); got != cobotDir {
		t.Errorf("expected %s, got %s", cobotDir, got)
	}
}

func TestDataDirDefault(t *testing.T) {
	os.Unsetenv("COBOT_DATA_PATH")
	os.Unsetenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "cobot")
	if got := DataDir(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestDataDirFromCobotEnv(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("COBOT_DATA_PATH", dir)
	defer os.Unsetenv("COBOT_DATA_PATH")
	if got := DataDir(); got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestDataDirFromXDG(t *testing.T) {
	os.Unsetenv("COBOT_DATA_PATH")
	dir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", dir)
	defer os.Unsetenv("XDG_DATA_HOME")
	expected := filepath.Join(dir, "cobot")
	if got := DataDir(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestDataDirCobotOverridesXDG(t *testing.T) {
	cobotDir := t.TempDir()
	xdgDir := t.TempDir()
	os.Setenv("COBOT_DATA_PATH", cobotDir)
	os.Setenv("XDG_DATA_HOME", xdgDir)
	defer os.Unsetenv("COBOT_DATA_PATH")
	defer os.Unsetenv("XDG_DATA_HOME")
	if got := DataDir(); got != cobotDir {
		t.Errorf("expected %s, got %s", cobotDir, got)
	}
}

func TestGlobalConfigPath(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("COBOT_CONFIG_PATH", dir)
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	expected := filepath.Join(dir, "config.yaml")
	if got := GlobalConfigPath(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestWorkspaceDefinitionsDir(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("COBOT_CONFIG_PATH", dir)
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	expected := filepath.Join(dir, "workspaces")
	if got := WorkspaceDefinitionsDir(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestGlobalSkillsDir(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("COBOT_DATA_PATH", dir)
	defer os.Unsetenv("COBOT_DATA_PATH")
	expected := filepath.Join(dir, "skills")
	if got := GlobalSkillsDir(); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
