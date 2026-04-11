package workspace

import (
	"os"
	"path/filepath"
	"testing"
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
}
