package persona

import (
	"fmt"
	"os"

	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
)

const defaultSoulContent = `# SOUL

You are Cobot, a personal AI assistant.

## Voice
- Concise and direct
- Technical but accessible
- Use analogies when helpful

## Style
- Prefer code examples over explanations
- Always suggest best practices
- Ask clarifying questions when ambiguous
`

const defaultUserContent = `# USER

## Profile
- Name: User
- Role: Developer

## Preferences
- Likes learning new technologies
- Values clean code and documentation

## Work Style
- Prefers practical solutions
- Values efficiency
`

const defaultMemoryContent = `# MEMORY

This file contains consolidated memories from your conversations.

## Active Context
- Current focus areas
- Recent learnings

## Key Facts
- Important information to remember

## Projects
- Active projects and their status
`

// Persona manages the personal agent's persona files
type Persona struct {
	ConfigDir string
	DataDir   string
}

// New creates a new Persona instance
func New() *Persona {
	return &Persona{
		ConfigDir: xdg.CobotConfigDir(),
		DataDir:   xdg.CobotDataDir(),
	}
}

// EnsureFiles creates all persona files if they don't exist
func (p *Persona) EnsureFiles() error {
	if err := p.EnsureSoulFile(); err != nil {
		return fmt.Errorf("ensure soul file: %w", err)
	}
	if err := p.EnsureUserFile(); err != nil {
		return fmt.Errorf("ensure user file: %w", err)
	}
	if err := p.EnsureMemoryFile(); err != nil {
		return fmt.Errorf("ensure memory file: %w", err)
	}
	return nil
}

// EnsureSoulFile creates SOUL.md if it doesn't exist
func (p *Persona) EnsureSoulFile() error {
	path := workspace.GlobalSoulPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultSoulContent), 0644); err != nil {
			return fmt.Errorf("write soul file: %w", err)
		}
	}
	return nil
}

// EnsureUserFile creates USER.md if it doesn't exist
func (p *Persona) EnsureUserFile() error {
	path := workspace.GlobalUserPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultUserContent), 0644); err != nil {
			return fmt.Errorf("write user file: %w", err)
		}
	}
	return nil
}

// EnsureMemoryFile creates MEMORY.md if it doesn't exist
func (p *Persona) EnsureMemoryFile() error {
	path := workspace.GlobalMemoryMdPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultMemoryContent), 0644); err != nil {
			return fmt.Errorf("write memory file: %w", err)
		}
	}
	return nil
}

// LoadSoul reads the SOUL.md content
func (p *Persona) LoadSoul() (string, error) {
	path := workspace.GlobalSoulPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSoulContent, nil
		}
		return "", fmt.Errorf("read soul file: %w", err)
	}
	return string(content), nil
}

// LoadUser reads the USER.md content
func (p *Persona) LoadUser() (string, error) {
	path := workspace.GlobalUserPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultUserContent, nil
		}
		return "", fmt.Errorf("read user file: %w", err)
	}
	return string(content), nil
}

// LoadMemory reads the MEMORY.md content
func (p *Persona) LoadMemory() (string, error) {
	path := workspace.GlobalMemoryMdPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultMemoryContent, nil
		}
		return "", fmt.Errorf("read memory file: %w", err)
	}
	return string(content), nil
}

// SaveSoul writes the SOUL.md content
func (p *Persona) SaveSoul(content string) error {
	path := workspace.GlobalSoulPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write soul file: %w", err)
	}
	return nil
}

// SaveUser writes the USER.md content
func (p *Persona) SaveUser(content string) error {
	path := workspace.GlobalUserPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write user file: %w", err)
	}
	return nil
}

// SaveMemory writes the MEMORY.md content
func (p *Persona) SaveMemory(content string) error {
	path := workspace.GlobalMemoryMdPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write memory file: %w", err)
	}
	return nil
}

// GetSoulPath returns the path to SOUL.md
func (p *Persona) GetSoulPath() string {
	return workspace.GlobalSoulPath()
}

// GetUserPath returns the path to USER.md
func (p *Persona) GetUserPath() string {
	return workspace.GlobalUserPath()
}

// GetMemoryPath returns the path to MEMORY.md
func (p *Persona) GetMemoryPath() string {
	return workspace.GlobalMemoryMdPath()
}
