package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobot-agent/cobot/internal/xdg"
)

func TestDiscoverInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cobot"), 0755)
	os.WriteFile(filepath.Join(dir, ".cobot", "config.yaml"), []byte("model: openai:gpt-4o\n"), 0644)

	ws, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root != dir {
		t.Errorf("expected root %s, got %s", dir, ws.Root)
	}
	if ws.ConfigPath != filepath.Join(dir, ".cobot", "config.yaml") {
		t.Errorf("unexpected config path: %s", ws.ConfigPath)
	}
	if ws.DataDir != xdg.CobotDataDir() {
		t.Errorf("expected global DataDir %s, got %s", xdg.CobotDataDir(), ws.DataDir)
	}
}

func TestDiscoverInParentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cobot"), 0755)
	os.WriteFile(filepath.Join(dir, ".cobot", "config.yaml"), []byte("model: openai:gpt-4o\n"), 0644)

	subdir := filepath.Join(dir, "sub", "project")
	os.MkdirAll(subdir, 0755)

	ws, err := Discover(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root != dir {
		t.Errorf("expected root %s, got %s", dir, ws.Root)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Discover(dir)
	if err == nil {
		t.Error("expected error when no .cobot found")
	}
}

func TestInitWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root != dir {
		t.Errorf("expected root %s, got %s", dir, ws.Root)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cobot", "config.yaml")); os.IsNotExist(err) {
		t.Error("config.yaml not created")
	}
	if ws.DataDir != xdg.CobotDataDir() {
		t.Errorf("expected global DataDir, got %s", ws.DataDir)
	}
}

func TestInitWorkspaceCreatesDataDirs(t *testing.T) {
	dir := t.TempDir()
	ws, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ws.MemoryDir()); os.IsNotExist(err) {
		t.Error("memory dir not created")
	}
	if _, err := os.Stat(ws.SessionsDir()); os.IsNotExist(err) {
		t.Error("sessions dir not created")
	}
}

func TestXDGDataDirRespectsEnv(t *testing.T) {
	dataDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", dataDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cobot"), 0755)
	os.WriteFile(filepath.Join(dir, ".cobot", "config.yaml"), []byte("model: openai:gpt-4o\n"), 0644)

	ws, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dataDir, "cobot")
	if ws.DataDir != expected {
		t.Errorf("expected DataDir %s, got %s", expected, ws.DataDir)
	}
}
