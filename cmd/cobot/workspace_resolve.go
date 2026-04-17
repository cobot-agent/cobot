package main

import (
	"fmt"

	"github.com/cobot-agent/cobot/internal/workspace"
)

// resolveWorkspace creates a Manager and resolves the workspace by name or discovery.
// The name parameter defaults to the global workspacePath flag if empty.
// Falls back to discovery from startDir, then to "default".
func resolveWorkspace() (*workspace.Workspace, error) {
	name := workspacePath
	m, err := workspace.NewManager()
	if err != nil {
		return nil, fmt.Errorf("create workspace manager: %w", err)
	}
	return m.ResolveByNameOrDiscover(name, ".")
}
