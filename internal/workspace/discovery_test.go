package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_NoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := Discover(tmpDir)
	if err == nil {
		t.Fatal("expected error when no workspace found")
	}
}

func TestDiscover_FindsWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "project")
	cobotDir := filepath.Join(targetDir, ".cobot")

	if err := os.MkdirAll(cobotDir, 0755); err != nil {
		t.Fatalf("failed to create .cobot dir: %v", err)
	}

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	_, err = EnsureDefaultWorkspace()
	if err != nil {
		t.Fatalf("failed to ensure default workspace: %v", err)
	}

	ws, err := Discover(targetDir)
	if err != nil {
		ws, err = manager.CreateProject(targetDir)
		if err != nil {
			t.Fatalf("failed to create project workspace: %v", err)
		}
	}

	if ws.Type != WorkspaceTypeProject {
		t.Errorf("expected project type, got %s", ws.Type)
	}
}

func TestDiscoverOrDefault_ReturnsDefault(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := EnsureDefaultWorkspace()
	if err != nil {
		t.Fatalf("failed to ensure default workspace: %v", err)
	}

	ws, err := DiscoverOrDefault(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ws.IsDefault() {
		t.Error("expected default workspace when no project found")
	}
}

func TestWorkspace_ValidatePathWithinWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		ID:        "test-id",
		Name:      "test",
		Type:      WorkspaceTypeCustom,
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
		Root:      "",
	}

	os.MkdirAll(ws.ConfigDir, 0755)
	os.MkdirAll(ws.DataDir, 0755)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid config path",
			path:    filepath.Join(ws.ConfigDir, "SOUL.md"),
			wantErr: false,
		},
		{
			name:    "valid data path",
			path:    filepath.Join(ws.DataDir, "memory", "test.db"),
			wantErr: false,
		},
		{
			name:    "outside workspace",
			path:    filepath.Join(tmpDir, "..", "outside.txt"),
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
			err := ws.ValidatePathWithinWorkspace(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathWithinWorkspace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
