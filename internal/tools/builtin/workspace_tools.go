package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/tools"
	"github.com/cobot-agent/cobot/internal/workspace"
	cobot "github.com/cobot-agent/cobot/pkg"
	"gopkg.in/yaml.v3"
)

type WorkspaceConfigUpdateTool struct {
	workspace *workspace.Workspace
}

func (t *WorkspaceConfigUpdateTool) Name() string { return "workspace_config_update" }
func (t *WorkspaceConfigUpdateTool) Description() string {
	return "Update workspace configuration. Can modify enabled_mcp, enabled_skills, and sandbox settings."
}

func (t *WorkspaceConfigUpdateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"enabled_mcp": {"type": "array", "items": {"type": "string"}, "description": "List of MCP server names to enable"},
			"enabled_skills": {"type": "array", "items": {"type": "string"}, "description": "List of skill names to enable"},
			"sandbox_root": {"type": "string", "description": "Sandbox root directory"},
			"allow_paths": {"type": "array", "items": {"type": "string"}, "description": "Additional allowed paths"},
			"blocked_commands": {"type": "array", "items": {"type": "string"}, "description": "Blocked shell commands"}
		}
	}`)
}

func (t *WorkspaceConfigUpdateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		EnabledMCP      *[]string `json:"enabled_mcp"`
		EnabledSkills   *[]string `json:"enabled_skills"`
		SandboxRoot     *string   `json:"sandbox_root"`
		AllowPaths      *[]string `json:"allow_paths"`
		BlockedCommands *[]string `json:"blocked_commands"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	cfg := t.workspace.Config
	if params.EnabledMCP != nil {
		cfg.EnabledMCP = *params.EnabledMCP
	}
	if params.EnabledSkills != nil {
		cfg.EnabledSkills = *params.EnabledSkills
	}
	if params.SandboxRoot != nil {
		cfg.Sandbox.Root = *params.SandboxRoot
	}
	if params.AllowPaths != nil {
		cfg.Sandbox.AllowPaths = *params.AllowPaths
	}
	if params.BlockedCommands != nil {
		cfg.Sandbox.BlockedCommands = *params.BlockedCommands
	}

	if err := t.workspace.SaveConfig(); err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}
	return "workspace config updated", nil
}

type skillCreateArgs struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

type SkillCreateTool struct {
	workspace *workspace.Workspace
}

func (t *SkillCreateTool) Name() string { return "skill_create" }
func (t *SkillCreateTool) Description() string {
	return "Create a new skill in the workspace skills directory"
}

func (t *SkillCreateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Name of the skill"},
			"format": {"type": "string", "enum": ["yaml", "markdown"], "description": "Format of the skill file"},
			"content": {"type": "string", "description": "Content of the skill"}
		},
		"required": ["name", "format", "content"]
	}`)
}

func (t *SkillCreateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a skillCreateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if a.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if a.Format != "yaml" && a.Format != "markdown" {
		return "", fmt.Errorf("format must be \"yaml\" or \"markdown\"")
	}

	ext := "yaml"
	if a.Format == "markdown" {
		ext = "md"
	}

	dir := t.workspace.SkillsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create skills dir: %w", err)
	}

	filename := fmt.Sprintf("%s.%s", a.Name, ext)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(a.Content), 0644); err != nil {
		return "", fmt.Errorf("write skill file: %w", err)
	}
	return fmt.Sprintf("skill created: %s", filename), nil
}

type personaUpdateArgs struct {
	File    string `json:"file"`
	Content string `json:"content"`
}

type PersonaUpdateTool struct {
	workspace *workspace.Workspace
}

func (t *PersonaUpdateTool) Name() string        { return "persona_update" }
func (t *PersonaUpdateTool) Description() string { return "Update SOUL.md or USER.md persona files" }

func (t *PersonaUpdateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "enum": ["soul", "user"], "description": "Which persona file to update"},
			"content": {"type": "string", "description": "New content for the persona file"}
		},
		"required": ["file", "content"]
	}`)
}

func (t *PersonaUpdateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a personaUpdateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	var path string
	switch strings.ToLower(a.File) {
	case "soul":
		path = t.workspace.GetSoulPath()
	case "user":
		path = t.workspace.GetUserPath()
	default:
		return "", fmt.Errorf("file must be \"soul\" or \"user\"")
	}

	if err := os.WriteFile(path, []byte(a.Content), 0644); err != nil {
		return "", fmt.Errorf("write persona file: %w", err)
	}
	return fmt.Sprintf("%s updated", strings.ToLower(a.File)), nil
}

type AgentConfigUpdateTool struct {
	workspace *workspace.Workspace
}

func (t *AgentConfigUpdateTool) Name() string { return "agent_config_update" }
func (t *AgentConfigUpdateTool) Description() string {
	return "Update an agent's configuration file in the workspace"
}

func (t *AgentConfigUpdateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent": {"type": "string", "description": "Agent name"},
			"model": {"type": "string", "description": "LLM model override"},
			"system_prompt": {"type": "string", "description": "System prompt or .md file ref"},
			"enabled_mcp": {"type": "array", "items": {"type": "string"}, "description": "MCP servers to enable"},
			"enabled_skills": {"type": "array", "items": {"type": "string"}, "description": "Skills to enable"},
			"max_turns": {"type": "integer", "description": "Max turns override"}
		},
		"required": ["agent"]
	}`)
}

func (t *AgentConfigUpdateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Agent         string    `json:"agent"`
		Model         *string   `json:"model"`
		SystemPrompt  *string   `json:"system_prompt"`
		EnabledMCP    *[]string `json:"enabled_mcp"`
		EnabledSkills *[]string `json:"enabled_skills"`
		MaxTurns      *int      `json:"max_turns"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	path := filepath.Join(t.workspace.AgentsDir(), params.Agent+".yaml")
	cfg, err := agent.LoadAgentConfig(path)
	if err != nil {
		return "", fmt.Errorf("load agent config: %w", err)
	}

	if params.Model != nil {
		cfg.Model = *params.Model
	}
	if params.SystemPrompt != nil {
		cfg.SystemPrompt = *params.SystemPrompt
	}
	if params.EnabledMCP != nil {
		cfg.EnabledMCP = *params.EnabledMCP
	}
	if params.EnabledSkills != nil {
		cfg.EnabledSkills = *params.EnabledSkills
	}
	if params.MaxTurns != nil {
		cfg.MaxTurns = *params.MaxTurns
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal agent config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write agent config: %w", err)
	}
	return fmt.Sprintf("agent config updated: %s", params.Agent), nil
}

type SkillUpdateTool struct {
	workspace *workspace.Workspace
}

func (t *SkillUpdateTool) Name() string { return "skill_update" }
func (t *SkillUpdateTool) Description() string {
	return "Update an existing skill in the workspace skills directory"
}

func (t *SkillUpdateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Skill name"},
			"content": {"type": "string", "description": "New content for the skill file"}
		},
		"required": ["name", "content"]
	}`)
}

func (t *SkillUpdateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	dir := t.workspace.SkillsDir()
	var found string
	for _, ext := range []string{".yaml", ".yml", ".md"} {
		candidate := filepath.Join(dir, params.Name+ext)
		if _, err := os.Stat(candidate); err == nil {
			found = candidate
			break
		}
	}
	if found == "" {
		return "", fmt.Errorf("skill not found: %s", params.Name)
	}

	if err := os.WriteFile(found, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write skill file: %w", err)
	}
	return fmt.Sprintf("skill updated: %s", filepath.Base(found)), nil
}

func RegisterWorkspaceTools(registry *tools.Registry, ws *workspace.Workspace) {
	registry.Register(&WorkspaceConfigUpdateTool{workspace: ws})
	registry.Register(&SkillCreateTool{workspace: ws})
	registry.Register(&PersonaUpdateTool{workspace: ws})
	registry.Register(&AgentConfigUpdateTool{workspace: ws})
	registry.Register(&SkillUpdateTool{workspace: ws})
}

var (
	_ cobot.Tool = (*WorkspaceConfigUpdateTool)(nil)
	_ cobot.Tool = (*SkillCreateTool)(nil)
	_ cobot.Tool = (*PersonaUpdateTool)(nil)
	_ cobot.Tool = (*AgentConfigUpdateTool)(nil)
	_ cobot.Tool = (*SkillUpdateTool)(nil)
)
