package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir_Default(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Unsetenv("XDG_CONFIG_HOME")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "cobot")

	got := ConfigDir()
	if got != expected {
		t.Errorf("ConfigDir() = %q, want %q", got, expected)
	}
}

func TestConfigDir_CobotConfigPath(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("COBOT_CONFIG_PATH", "/custom/config")
	defer os.Unsetenv("COBOT_CONFIG_PATH")

	got := ConfigDir()
	if got != "/custom/config" {
		t.Errorf("ConfigDir() = %q, want %q", got, "/custom/config")
	}
}

func TestConfigDir_XDGConfigHome(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := ConfigDir()
	if got != "/xdg/config/cobot" {
		t.Errorf("ConfigDir() = %q, want %q", got, "/xdg/config/cobot")
	}
}

func TestConfigDir_CobotOverridesXDG(t *testing.T) {
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	os.Setenv("COBOT_CONFIG_PATH", "/custom/config")
	defer os.Unsetenv("XDG_CONFIG_HOME")
	defer os.Unsetenv("COBOT_CONFIG_PATH")

	got := ConfigDir()
	if got != "/custom/config" {
		t.Errorf("ConfigDir() = %q, want %q", got, "/custom/config")
	}
}

func TestDataDir_Default(t *testing.T) {
	os.Unsetenv("COBOT_DATA_PATH")
	os.Unsetenv("XDG_DATA_HOME")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "cobot")

	got := DataDir()
	if got != expected {
		t.Errorf("DataDir() = %q, want %q", got, expected)
	}
}

func TestDataDir_CobotDataPath(t *testing.T) {
	os.Unsetenv("XDG_DATA_HOME")
	os.Setenv("COBOT_DATA_PATH", "/custom/data")
	defer os.Unsetenv("COBOT_DATA_PATH")

	got := DataDir()
	if got != "/custom/data" {
		t.Errorf("DataDir() = %q, want %q", got, "/custom/data")
	}
}

func TestDataDir_XDGDataHome(t *testing.T) {
	os.Unsetenv("COBOT_DATA_PATH")
	os.Setenv("XDG_DATA_HOME", "/xdg/data")
	defer os.Unsetenv("XDG_DATA_HOME")

	got := DataDir()
	if got != "/xdg/data/cobot" {
		t.Errorf("DataDir() = %q, want %q", got, "/xdg/data/cobot")
	}
}

func TestDataDir_CobotOverridesXDG(t *testing.T) {
	os.Setenv("XDG_DATA_HOME", "/xdg/data")
	os.Setenv("COBOT_DATA_PATH", "/custom/data")
	defer os.Unsetenv("XDG_DATA_HOME")
	defer os.Unsetenv("COBOT_DATA_PATH")

	got := DataDir()
	if got != "/custom/data" {
		t.Errorf("DataDir() = %q, want %q", got, "/custom/data")
	}
}

func TestGlobalConfigPath(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Setenv("XDG_CONFIG_HOME", "/test")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := GlobalConfigPath()
	if got != "/test/cobot/config.yaml" {
		t.Errorf("GlobalConfigPath() = %q, want %q", got, "/test/cobot/config.yaml")
	}
}

func TestMCPRegistryDir(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Setenv("XDG_CONFIG_HOME", "/test")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := MCPRegistryDir()
	if got != "/test/cobot/mcp" {
		t.Errorf("MCPRegistryDir() = %q, want %q", got, "/test/cobot/mcp")
	}
}

func TestSkillsRegistryDir(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Setenv("XDG_CONFIG_HOME", "/test")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := SkillsRegistryDir()
	if got != "/test/cobot/skills" {
		t.Errorf("SkillsRegistryDir() = %q, want %q", got, "/test/cobot/skills")
	}
}

func TestWorkspaceDefinitionsDir(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Setenv("XDG_CONFIG_HOME", "/test")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := WorkspaceDefinitionsDir()
	if got != "/test/cobot/workspaces" {
		t.Errorf("WorkspaceDefinitionsDir() = %q, want %q", got, "/test/cobot/workspaces")
	}
}

func TestDerivedDirs_UseCobotConfigPath(t *testing.T) {
	os.Setenv("COBOT_CONFIG_PATH", "/my/config")
	defer os.Unsetenv("COBOT_CONFIG_PATH")

	if got := GlobalConfigPath(); got != "/my/config/config.yaml" {
		t.Errorf("GlobalConfigPath() = %q, want %q", got, "/my/config/config.yaml")
	}
	if got := MCPRegistryDir(); got != "/my/config/mcp" {
		t.Errorf("MCPRegistryDir() = %q, want %q", got, "/my/config/mcp")
	}
	if got := SkillsRegistryDir(); got != "/my/config/skills" {
		t.Errorf("SkillsRegistryDir() = %q, want %q", got, "/my/config/skills")
	}
	if got := WorkspaceDefinitionsDir(); got != "/my/config/workspaces" {
		t.Errorf("WorkspaceDefinitionsDir() = %q, want %q", got, "/my/config/workspaces")
	}
}
