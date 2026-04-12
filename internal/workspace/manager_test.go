package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobot-agent/cobot/internal/xdg"
)

func setupTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origConfig := os.Getenv("COBOT_CONFIG_PATH")
	origData := os.Getenv("COBOT_DATA_PATH")

	os.Setenv("COBOT_CONFIG_PATH", filepath.Join(tmpDir, "config"))
	os.Setenv("COBOT_DATA_PATH", filepath.Join(tmpDir, "data"))

	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	cleanup := func() {
		if origConfig != "" {
			os.Setenv("COBOT_CONFIG_PATH", origConfig)
		} else {
			os.Unsetenv("COBOT_CONFIG_PATH")
		}
		if origData != "" {
			os.Setenv("COBOT_DATA_PATH", origData)
		} else {
			os.Unsetenv("COBOT_DATA_PATH")
		}
	}

	return m, cleanup
}

func TestNewManager_CreatesDefaultWorkspace(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	ws, err := m.Resolve("default")
	if err != nil {
		t.Fatalf("Resolve(default) error: %v", err)
	}
	if !ws.IsDefault() {
		t.Error("default workspace should be of default type")
	}
	if ws.Config.ID == "" {
		t.Error("default workspace should have an ID")
	}
}

func TestManager_Create_Custom(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	ws, err := m.Create("myproject", WorkspaceTypeCustom, "", "")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if ws.Definition.Name != "myproject" {
		t.Errorf("Name = %s, want myproject", ws.Definition.Name)
	}
	if ws.Definition.Type != WorkspaceTypeCustom {
		t.Errorf("Type = %s, want custom", ws.Definition.Type)
	}
	if ws.Config.ID == "" {
		t.Error("workspace config should have an ID")
	}

	if _, err := os.Stat(ws.ConfigPath()); os.IsNotExist(err) {
		t.Error("workspace config file should exist")
	}
}

func TestManager_Create_WithCustomPath(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	customPath := filepath.Join(t.TempDir(), "custom-data")
	ws, err := m.Create("custom1", WorkspaceTypeCustom, "", customPath)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if ws.DataDir != customPath {
		t.Errorf("DataDir = %s, want %s", ws.DataDir, customPath)
	}
}

func TestManager_Create_DuplicateName(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := m.Create("dup", WorkspaceTypeCustom, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Create("dup", WorkspaceTypeCustom, "", "")
	if err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestManager_Create_ReservedName(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := m.Create("default", WorkspaceTypeCustom, "", "")
	if err == nil {
		t.Error("expected error for reserved name 'default'")
	}
}

func TestManager_List(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := m.Create("ws1", WorkspaceTypeCustom, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Create("ws2", WorkspaceTypeCustom, "", "")
	if err != nil {
		t.Fatal(err)
	}

	defs, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	if !names["default"] {
		t.Error("List() should include default workspace")
	}
	if !names["ws1"] {
		t.Error("List() should include ws1")
	}
	if !names["ws2"] {
		t.Error("List() should include ws2")
	}
}

func TestManager_Delete(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := m.Create("todelete", WorkspaceTypeCustom, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Delete("todelete"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = m.Resolve("todelete")
	if err == nil {
		t.Error("expected error resolving deleted workspace")
	}
}

func TestManager_Delete_Default(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	err := m.Delete("default")
	if err == nil {
		t.Error("expected error deleting default workspace")
	}
}

func TestManager_Resolve(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	ws, err := m.Resolve("default")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if ws.Definition.Name != "default" {
		t.Errorf("Name = %s, want default", ws.Definition.Name)
	}
	expectedDataDir := filepath.Join(xdg.DataDir(), "default")
	if ws.DataDir != expectedDataDir {
		t.Errorf("DataDir = %s, want %s", ws.DataDir, expectedDataDir)
	}
}

func TestManager_Resolve_NotFound(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := m.Resolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestManager_Discover(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	projectDir := filepath.Join(t.TempDir(), "myproject")
	cobotDir := filepath.Join(projectDir, ".cobot")
	if err := os.MkdirAll(cobotDir, 0755); err != nil {
		t.Fatal(err)
	}

	ws, err := m.Discover(projectDir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if !ws.IsProject() {
		t.Error("discovered workspace should be project type")
	}
	if ws.Definition.Root != projectDir {
		t.Errorf("Root = %s, want %s", ws.Definition.Root, projectDir)
	}
}

func TestManager_Discover_NotFound(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	tmpDir := t.TempDir()
	_, err := m.Discover(tmpDir)
	if err == nil {
		t.Error("expected error when no .cobot directory found")
	}
}

func TestManager_ResolveByNameOrDiscover_WithName(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	_, err := m.Create("named-ws", WorkspaceTypeCustom, "", "")
	if err != nil {
		t.Fatal(err)
	}

	ws, err := m.ResolveByNameOrDiscover("named-ws", ".")
	if err != nil {
		t.Fatalf("ResolveByNameOrDiscover() error: %v", err)
	}
	if ws.Definition.Name != "named-ws" {
		t.Errorf("Name = %s, want named-ws", ws.Definition.Name)
	}
}

func TestManager_ResolveByNameOrDiscover_NoNameFallsToDefault(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	ws, err := m.ResolveByNameOrDiscover("", t.TempDir())
	if err != nil {
		t.Fatalf("ResolveByNameOrDiscover() error: %v", err)
	}
	if !ws.IsDefault() {
		t.Error("expected default workspace when no name and no .cobot found")
	}
}
