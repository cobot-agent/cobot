package persona

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}
	if p.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
}

func TestEnsureFiles(t *testing.T) {
	if err := workspace.EnsureGlobalWorkspace(); err != nil {
		t.Fatalf("Failed to ensure global workspace: %v", err)
	}

	p := New()

	os.Remove(p.GetSoulPath())
	os.Remove(p.GetUserPath())
	os.Remove(p.GetMemoryPath())

	if err := p.EnsureFiles(); err != nil {
		t.Fatalf("EnsureFiles failed: %v", err)
	}

	if _, err := os.Stat(p.GetSoulPath()); err != nil {
		t.Errorf("SOUL.md was not created: %v", err)
	}
	if _, err := os.Stat(p.GetUserPath()); err != nil {
		t.Errorf("USER.md was not created: %v", err)
	}
	if _, err := os.Stat(p.GetMemoryPath()); err != nil {
		t.Errorf("MEMORY.md was not created: %v", err)
	}
}

func TestLoadAndSave(t *testing.T) {
	if err := workspace.EnsureGlobalWorkspace(); err != nil {
		t.Fatalf("Failed to ensure global workspace: %v", err)
	}

	p := New()
	p.EnsureFiles()

	testSoul := "# Test SOUL\n\nTest content"
	if err := p.SaveSoul(testSoul); err != nil {
		t.Errorf("SaveSoul failed: %v", err)
	}

	loadedSoul, err := p.LoadSoul()
	if err != nil {
		t.Errorf("LoadSoul failed: %v", err)
	}
	if loadedSoul != testSoul {
		t.Errorf("LoadSoul returned wrong content, got:\n%s\nwant:\n%s", loadedSoul, testSoul)
	}

	testUser := "# Test USER\n\nTest content"
	if err := p.SaveUser(testUser); err != nil {
		t.Errorf("SaveUser failed: %v", err)
	}

	loadedUser, err := p.LoadUser()
	if err != nil {
		t.Errorf("LoadUser failed: %v", err)
	}
	if loadedUser != testUser {
		t.Errorf("LoadUser returned wrong content, got:\n%s\nwant:\n%s", loadedUser, testUser)
	}

	testMemory := "# Test MEMORY\n\nTest content"
	if err := p.SaveMemory(testMemory); err != nil {
		t.Errorf("SaveMemory failed: %v", err)
	}

	loadedMemory, err := p.LoadMemory()
	if err != nil {
		t.Errorf("LoadMemory failed: %v", err)
	}
	if loadedMemory != testMemory {
		t.Errorf("LoadMemory returned wrong content, got:\n%s\nwant:\n%s", loadedMemory, testMemory)
	}
}

func TestLoadDefaults(t *testing.T) {
	p := New()

	os.Remove(p.GetSoulPath())
	os.Remove(p.GetUserPath())
	os.Remove(p.GetMemoryPath())

	soul, err := p.LoadSoul()
	if err != nil {
		t.Errorf("LoadSoul with missing file failed: %v", err)
	}
	if soul == "" {
		t.Error("LoadSoul should return default content when file doesn't exist")
	}

	user, err := p.LoadUser()
	if err != nil {
		t.Errorf("LoadUser with missing file failed: %v", err)
	}
	if user == "" {
		t.Error("LoadUser should return default content when file doesn't exist")
	}

	memory, err := p.LoadMemory()
	if err != nil {
		t.Errorf("LoadMemory with missing file failed: %v", err)
	}
	if memory == "" {
		t.Error("LoadMemory should return default content when file doesn't exist")
	}
}

func TestGetPaths(t *testing.T) {
	p := New()

	soulPath := p.GetSoulPath()
	if soulPath == "" {
		t.Error("GetSoulPath returned empty string")
	}
	if !filepath.IsAbs(soulPath) {
		t.Errorf("GetSoulPath should return absolute path, got: %s", soulPath)
	}

	userPath := p.GetUserPath()
	if userPath == "" {
		t.Error("GetUserPath returned empty string")
	}

	memoryPath := p.GetMemoryPath()
	if memoryPath == "" {
		t.Error("GetMemoryPath returned empty string")
	}
}

func TestFileLocations(t *testing.T) {
	p := New()

	configDir := xdg.CobotConfigDir()

	if p.GetSoulPath() != filepath.Join(configDir, "SOUL.md") {
		t.Errorf("SOUL.md should be in config dir")
	}

	if p.GetUserPath() != filepath.Join(configDir, "USER.md") {
		t.Errorf("USER.md should be in config dir")
	}

	if p.GetMemoryPath() != filepath.Join(configDir, "MEMORY.md") {
		t.Errorf("MEMORY.md should be in config dir")
	}
}
