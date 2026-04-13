package persona

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobot-agent/cobot/internal/workspace"
)

func setupTestWorkspace(t *testing.T) *workspace.Workspace {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Definition: &workspace.WorkspaceDefinition{
			Name: "test",
			Type: workspace.WorkspaceTypeCustom,
		},
		Config: &workspace.WorkspaceConfig{
			ID:   "test-id",
			Name: "test",
			Type: workspace.WorkspaceTypeCustom,
		},
		DataDir: filepath.Join(tmpDir, "data"),
	}
	if err := ws.EnsureDirs(); err != nil {
		t.Fatalf("Failed to create workspace dirs: %v", err)
	}
	return ws
}

func TestNewService(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)

	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
	if svc.ws != ws {
		t.Error("Service should hold reference to workspace")
	}
}

func TestEnsureFiles(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)

	os.Remove(ws.GetSoulPath())
	os.Remove(ws.GetUserPath())
	os.Remove(ws.GetMemoryMdPath())

	if err := svc.EnsureFiles(); err != nil {
		t.Fatalf("EnsureFiles failed: %v", err)
	}

	if _, err := os.Stat(ws.GetSoulPath()); err != nil {
		t.Errorf("SOUL.md was not created: %v", err)
	}
	if _, err := os.Stat(ws.GetUserPath()); err != nil {
		t.Errorf("USER.md was not created: %v", err)
	}
	if _, err := os.Stat(ws.GetMemoryMdPath()); err != nil {
		t.Errorf("MEMORY.md was not created: %v", err)
	}
}

func TestLoadAndSave(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)
	svc.EnsureFiles()

	testSoul := "# Test SOUL\n\nTest content"
	if err := svc.SaveSoul(testSoul); err != nil {
		t.Errorf("SaveSoul failed: %v", err)
	}

	loadedSoul, err := svc.LoadSoul()
	if err != nil {
		t.Errorf("LoadSoul failed: %v", err)
	}
	if loadedSoul != testSoul {
		t.Errorf("LoadSoul returned wrong content, got:\n%s\nwant:\n%s", loadedSoul, testSoul)
	}

	testUser := "# Test USER\n\nTest content"
	if err := svc.SaveUser(testUser); err != nil {
		t.Errorf("SaveUser failed: %v", err)
	}

	loadedUser, err := svc.LoadUser()
	if err != nil {
		t.Errorf("LoadUser failed: %v", err)
	}
	if loadedUser != testUser {
		t.Errorf("LoadUser returned wrong content, got:\n%s\nwant:\n%s", loadedUser, testUser)
	}

	testMemory := "# Test MEMORY\n\nTest content"
	if err := svc.SaveMemory(testMemory); err != nil {
		t.Errorf("SaveMemory failed: %v", err)
	}

	loadedMemory, err := svc.LoadMemory()
	if err != nil {
		t.Errorf("LoadMemory failed: %v", err)
	}
	if loadedMemory != testMemory {
		t.Errorf("LoadMemory returned wrong content, got:\n%s\nwant:\n%s", loadedMemory, testMemory)
	}
}

func TestLoadDefaults(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)

	os.Remove(ws.GetSoulPath())
	os.Remove(ws.GetUserPath())
	os.Remove(ws.GetMemoryMdPath())

	soul, err := svc.LoadSoul()
	if err != nil {
		t.Errorf("LoadSoul with missing file failed: %v", err)
	}
	if soul == "" {
		t.Error("LoadSoul should return default content when file doesn't exist")
	}

	user, err := svc.LoadUser()
	if err != nil {
		t.Errorf("LoadUser with missing file failed: %v", err)
	}
	if user == "" {
		t.Error("LoadUser should return default content when file doesn't exist")
	}

	memory, err := svc.LoadMemory()
	if err != nil {
		t.Errorf("LoadMemory with missing file failed: %v", err)
	}
	if memory == "" {
		t.Error("LoadMemory should return default content when file doesn't exist")
	}
}

func TestGetPaths(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)

	soulPath := svc.GetSoulPath()
	if soulPath == "" {
		t.Error("GetSoulPath returned empty string")
	}
	if !filepath.IsAbs(soulPath) {
		t.Errorf("GetSoulPath should return absolute path, got: %s", soulPath)
	}

	userPath := svc.GetUserPath()
	if userPath == "" {
		t.Error("GetUserPath returned empty string")
	}

	memoryPath := svc.GetMemoryPath()
	if memoryPath == "" {
		t.Error("GetMemoryPath returned empty string")
	}
}

func TestFileLocations(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)

	if svc.GetSoulPath() != filepath.Join(ws.DataDir, "SOUL.md") {
		t.Errorf("SOUL.md should be in workspace data dir")
	}

	if svc.GetUserPath() != filepath.Join(ws.DataDir, "USER.md") {
		t.Errorf("USER.md should be in workspace data dir")
	}

	if svc.GetMemoryPath() != filepath.Join(ws.DataDir, "MEMORY.md") {
		t.Errorf("MEMORY.md should be in workspace data dir")
	}
}

func TestWorkspaceReference(t *testing.T) {
	ws := setupTestWorkspace(t)
	svc := NewService(ws)

	if svc.Workspace() != ws {
		t.Error("Workspace() should return the workspace reference")
	}
}
