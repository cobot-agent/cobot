package persona

import (
	"fmt"
	"os"

	"github.com/cobot-agent/cobot/internal/workspace"
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

type Service struct {
	ws *workspace.Workspace
}

func NewService(ws *workspace.Workspace) *Service {
	return &Service{ws: ws}
}

func (s *Service) EnsureFiles() error {
	if err := s.EnsureSoulFile(); err != nil {
		return fmt.Errorf("ensure soul file: %w", err)
	}
	if err := s.EnsureUserFile(); err != nil {
		return fmt.Errorf("ensure user file: %w", err)
	}
	if err := s.EnsureMemoryFile(); err != nil {
		return fmt.Errorf("ensure memory file: %w", err)
	}
	return nil
}

func (s *Service) ensureFile(path string, content string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) EnsureSoulFile() error {
	return s.ensureFile(s.ws.GetSoulPath(), defaultSoulContent)
}

func (s *Service) EnsureUserFile() error {
	return s.ensureFile(s.ws.GetUserPath(), defaultUserContent)
}

func (s *Service) EnsureMemoryFile() error {
	return s.ensureFile(s.ws.GetMemoryMdPath(), defaultMemoryContent)
}

func (s *Service) LoadSoul() (string, error) {
	return s.loadFile(s.ws.GetSoulPath(), defaultSoulContent)
}

func (s *Service) LoadUser() (string, error) {
	return s.loadFile(s.ws.GetUserPath(), defaultUserContent)
}

func (s *Service) LoadMemory() (string, error) {
	return s.loadFile(s.ws.GetMemoryMdPath(), defaultMemoryContent)
}

func (s *Service) loadFile(path string, defaultContent string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultContent, nil
		}
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(content), nil
}

func (s *Service) SaveSoul(content string) error {
	return s.saveFile(s.ws.GetSoulPath(), content)
}

func (s *Service) SaveUser(content string) error {
	return s.saveFile(s.ws.GetUserPath(), content)
}

func (s *Service) SaveMemory(content string) error {
	return s.saveFile(s.ws.GetMemoryMdPath(), content)
}

func (s *Service) saveFile(path string, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *Service) GetSoulPath() string {
	return s.ws.GetSoulPath()
}

func (s *Service) GetUserPath() string {
	return s.ws.GetUserPath()
}

func (s *Service) GetMemoryPath() string {
	return s.ws.GetMemoryMdPath()
}

func (s *Service) Workspace() *workspace.Workspace {
	return s.ws
}
