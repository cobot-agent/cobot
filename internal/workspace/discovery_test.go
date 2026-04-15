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

	ws, err := manager.Discover(targetDir)
	if err != nil {
		t.Fatalf("failed to discover project workspace: %v", err)
	}

	if ws.Definition.Type != WorkspaceTypeProject {
		t.Errorf("expected project type, got %s", ws.Definition.Type)
	}
}

func TestDiscoverOrDefault_ReturnsDefault(t *testing.T) {
	tmpDir := t.TempDir()

	ws, err := DiscoverOrDefault(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ws.IsDefault() {
		t.Error("expected default workspace when no project found")
	}
}

func TestWorkspace_ValidatePath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &Workspace{
		Definition: &WorkspaceDefinition{
			Name: "test",
			Type: WorkspaceTypeCustom,
		},
		Config: &WorkspaceConfig{
			ID:   "test-id",
			Name: "test",
			Type: WorkspaceTypeCustom,
		},
		DataDir: filepath.Join(tmpDir, "data"),
	}

	os.MkdirAll(ws.DataDir, 0755)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid data path",
			path:    filepath.Join(ws.DataDir, "SOUL.md"),
			wantErr: false,
		},
		{
			name:    "valid nested data path",
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
			err := ws.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
