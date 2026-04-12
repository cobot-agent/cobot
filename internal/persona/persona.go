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

func (s *Service) EnsureSoulFile() error {
	path := s.ws.GetSoulPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultSoulContent), 0644); err != nil {
			return fmt.Errorf("write soul file: %w", err)
		}
	}
	return nil
}

func (s *Service) EnsureUserFile() error {
	path := s.ws.GetUserPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultUserContent), 0644); err != nil {
			return fmt.Errorf("write user file: %w", err)
		}
	}
	return nil
}

func (s *Service) EnsureMemoryFile() error {
	path := s.ws.GetMemoryMdPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultMemoryContent), 0644); err != nil {
			return fmt.Errorf("write memory file: %w", err)
		}
	}
	return nil
}

func (s *Service) LoadSoul() (string, error) {
	path := s.ws.GetSoulPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSoulContent, nil
		}
		return "", fmt.Errorf("read soul file: %w", err)
	}
	return string(content), nil
}

func (s *Service) LoadUser() (string, error) {
	path := s.ws.GetUserPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultUserContent, nil
		}
		return "", fmt.Errorf("read user file: %w", err)
	}
	return string(content), nil
}

func (s *Service) LoadMemory() (string, error) {
	path := s.ws.GetMemoryMdPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultMemoryContent, nil
		}
		return "", fmt.Errorf("read memory file: %w", err)
	}
	return string(content), nil
}

func (s *Service) SaveSoul(content string) error {
	path := s.ws.GetSoulPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write soul file: %w", err)
	}
	return nil
}

func (s *Service) SaveUser(content string) error {
	path := s.ws.GetUserPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write user file: %w", err)
	}
	return nil
}

func (s *Service) SaveMemory(content string) error {
	path := s.ws.GetMemoryMdPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write memory file: %w", err)
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
