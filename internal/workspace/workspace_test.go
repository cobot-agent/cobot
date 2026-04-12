package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceDefinition_ResolvePath_Default(t *testing.T) {
	d := &WorkspaceDefinition{Name: "myproject"}
	result := d.ResolvePath("/data")
	expected := filepath.Join("/data", "myproject")
	if result != expected {
		t.Errorf("ResolvePath() = %s, want %s", result, expected)
	}
}

func TestWorkspaceDefinition_ResolvePath_CustomPath(t *testing.T) {
	d := &WorkspaceDefinition{Name: "myproject", Path: "/custom/location"}
	result := d.ResolvePath("/data")
	if result != "/custom/location" {
		t.Errorf("ResolvePath() = %s, want /custom/location", result)
	}
}

func TestSaveAndLoadDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	defPath := filepath.Join(tmpDir, "test.yaml")

	original := &WorkspaceDefinition{
		Name: "test-ws",
		Type: WorkspaceTypeCustom,
		Path: "/custom/path",
		Root: "/project/root",
	}

	if err := saveDefinition(original, defPath); err != nil {
		t.Fatalf("saveDefinition() error: %v", err)
	}

	loaded, err := loadDefinition(defPath)
	if err != nil {
		t.Fatalf("loadDefinition() error: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name = %s, want %s", loaded.Name, original.Name)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type = %s, want %s", loaded.Type, original.Type)
	}
	if loaded.Path != original.Path {
		t.Errorf("Path = %s, want %s", loaded.Path, original.Path)
	}
	if loaded.Root != original.Root {
		t.Errorf("Root = %s, want %s", loaded.Root, original.Root)
	}
}

func TestSaveAndLoadWorkspaceConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		Definition: &WorkspaceDefinition{Name: "test", Type: WorkspaceTypeCustom},
		Config:     newWorkspaceConfig("test", WorkspaceTypeCustom, "/project/root"),
		DataDir:    filepath.Join(tmpDir, "ws-data"),
	}

	if err := ws.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	if err := ws.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	loaded, err := loadWorkspaceConfig(ws.ConfigPath())
	if err != nil {
		t.Fatalf("loadWorkspaceConfig() error: %v", err)
	}

	if loaded.ID != ws.Config.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, ws.Config.ID)
	}
	if loaded.Name != ws.Config.Name {
		t.Errorf("Name = %s, want %s", loaded.Name, ws.Config.Name)
	}
	if loaded.Type != ws.Config.Type {
		t.Errorf("Type = %s, want %s", loaded.Type, ws.Config.Type)
	}
	if loaded.Root != ws.Config.Root {
		t.Errorf("Root = %s, want %s", loaded.Root, ws.Config.Root)
	}
}

func TestNewWorkspaceConfig(t *testing.T) {
	cfg := newWorkspaceConfig("test", WorkspaceTypeProject, "/project")
	if cfg.ID == "" {
		t.Error("ID should not be empty")
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %s, want test", cfg.Name)
	}
	if cfg.Type != WorkspaceTypeProject {
		t.Errorf("Type = %s, want project", cfg.Type)
	}
	if cfg.Root != "/project" {
		t.Errorf("Root = %s, want /project", cfg.Root)
	}
	if cfg.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestWorkspace_IsDefault(t *testing.T) {
	ws := &Workspace{
		Definition: &WorkspaceDefinition{Type: WorkspaceTypeDefault},
	}
	if !ws.IsDefault() {
		t.Error("IsDefault() should return true for default type")
	}

	ws.Definition.Type = WorkspaceTypeProject
	if ws.IsDefault() {
		t.Error("IsDefault() should return false for project type")
	}
}

func TestWorkspace_IsProject(t *testing.T) {
	ws := &Workspace{
		Definition: &WorkspaceDefinition{Type: WorkspaceTypeProject},
	}
	if !ws.IsProject() {
		t.Error("IsProject() should return true for project type")
	}
}

func TestWorkspace_Dirs(t *testing.T) {
	ws := &Workspace{
		DataDir: "/data/myworkspace",
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"MemoryDir", ws.MemoryDir(), "/data/myworkspace/memory"},
		{"SessionsDir", ws.SessionsDir(), "/data/myworkspace/sessions"},
		{"SkillsDir", ws.SkillsDir(), "/data/myworkspace/skills"},
		{"SchedulerDir", ws.SchedulerDir(), "/data/myworkspace/scheduler"},
		{"AgentsDir", ws.AgentsDir(), "/data/myworkspace/agents"},
		{"GetSoulPath", ws.GetSoulPath(), "/data/myworkspace/SOUL.md"},
		{"GetUserPath", ws.GetUserPath(), "/data/myworkspace/USER.md"},
		{"GetMemoryMdPath", ws.GetMemoryMdPath(), "/data/myworkspace/MEMORY.md"},
		{"ConfigPath", ws.ConfigPath(), "/data/myworkspace/workspace.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %s, want %s", tt.got, tt.want)
			}
		})
	}
}

func TestWorkspace_AgentsMdPath(t *testing.T) {
	ws := &Workspace{
		Definition: &WorkspaceDefinition{Root: "/project"},
	}
	expected := filepath.Join("/project", ".cobot", "AGENTS.md")
	if ws.AgentsMdPath() != expected {
		t.Errorf("AgentsMdPath() = %s, want %s", ws.AgentsMdPath(), expected)
	}

	ws.Definition.Root = ""
	if ws.AgentsMdPath() != "" {
		t.Errorf("AgentsMdPath() should return empty string when no root")
	}
}

func TestWorkspace_EnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		DataDir: filepath.Join(tmpDir, "ws-data"),
	}

	if err := ws.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error: %v", err)
	}

	dirs := []string{
		ws.DataDir,
		ws.MemoryDir(),
		ws.SessionsDir(),
		ws.SkillsDir(),
		ws.AgentsDir(),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", dir)
		}
	}
}

func TestWorkspace_SaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		Definition: &WorkspaceDefinition{Name: "test", Type: WorkspaceTypeCustom},
		Config:     newWorkspaceConfig("test", WorkspaceTypeCustom, ""),
		DataDir:    filepath.Join(tmpDir, "ws-data"),
	}

	if err := ws.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	if err := ws.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	if _, err := os.Stat(ws.ConfigPath()); os.IsNotExist(err) {
		t.Error("config file not created")
	}

	loaded, err := loadWorkspaceConfig(ws.ConfigPath())
	if err != nil {
		t.Fatalf("loadWorkspaceConfig() error: %v", err)
	}
	if loaded.ID != ws.Config.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, ws.Config.ID)
	}
}

func TestWorkspace_ValidatePath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		Definition: &WorkspaceDefinition{Name: "test", Type: WorkspaceTypeCustom},
		DataDir:    filepath.Join(tmpDir, "data"),
	}
	os.MkdirAll(ws.DataDir, 0755)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid data path",
			path:    filepath.Join(ws.DataDir, "test.db"),
			wantErr: false,
		},
		{
			name:    "outside workspace",
			path:    filepath.Join(tmpDir, "outside.txt"),
			wantErr: true,
		},
		{
			name:    "system path",
			path:    "/etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ws.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkspace_ValidatePath_WithRoot(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectDir, 0755)

	ws := &Workspace{
		Definition: &WorkspaceDefinition{Name: "proj", Type: WorkspaceTypeProject, Root: projectDir},
		DataDir:    filepath.Join(tmpDir, "data"),
	}
	os.MkdirAll(ws.DataDir, 0755)

	err := ws.ValidatePath(filepath.Join(projectDir, "src", "main.go"))
	if err != nil {
		t.Errorf("ValidatePath() within project root should pass, got: %v", err)
	}

	err = ws.ValidatePath("/etc/passwd")
	if err == nil {
		t.Error("ValidatePath() outside all boundaries should fail")
	}
}
