package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func setupDiscoveryManager(t *testing.T) (*Manager, func()) {
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

func TestDiscover_NoWorkspace(t *testing.T) {
	_, cleanup := setupDiscoveryManager(t)
	defer cleanup()

	tmpDir := t.TempDir()
	_, err := Discover(tmpDir)
	if err == nil {
		t.Fatal("expected error when no workspace found")
	}
}

func TestDiscover_FindsWorkspace(t *testing.T) {
	m, cleanup := setupDiscoveryManager(t)
	defer cleanup()

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "project")
	cobotDir := filepath.Join(targetDir, ".cobot")

	if err := os.MkdirAll(cobotDir, 0755); err != nil {
		t.Fatalf("failed to create .cobot dir: %v", err)
	}

	ws, err := m.Discover(targetDir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if ws.Definition.Type != WorkspaceTypeProject {
		t.Errorf("expected project type, got %s", ws.Definition.Type)
	}
}

func TestDiscoverOrDefault_ReturnsDefault(t *testing.T) {
	_, cleanup := setupDiscoveryManager(t)
	defer cleanup()

	tmpDir := t.TempDir()
	ws, err := DiscoverOrDefault(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ws.IsDefault() {
		t.Error("expected default workspace when no project found")
	}
}
