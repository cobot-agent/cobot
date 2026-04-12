package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/llm/anthropic"
	"github.com/cobot-agent/cobot/internal/llm/openai"
	"github.com/cobot-agent/cobot/internal/mcp"
	"github.com/cobot-agent/cobot/internal/memory/daemon"
	"github.com/cobot-agent/cobot/internal/skills"
	"github.com/cobot-agent/cobot/internal/tools/builtin"
	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func parseModel(model string) (providerName, modelName string) {
	parts := strings.SplitN(model, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "openai", model
}

func initProvider(cfg *cobot.Config) (cobot.Provider, error) {
	providerName, modelName := parseModel(cfg.Model)
	cfg.Model = modelName

	apiKey := cfg.APIKeys[providerName]
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key not configured\n\nSet it via:\n  export %s_API_KEY=sk-...\n  # or in config: api_keys.%s: sk-...", providerName, strings.ToUpper(providerName), providerName)
	}

	baseURL := ""
	if cfg.Providers != nil {
		if pc, ok := cfg.Providers[providerName]; ok {
			baseURL = pc.BaseURL
		}
	}

	switch providerName {
	case "anthropic":
		return anthropic.NewProvider(apiKey, baseURL), nil
	case "openai":
		return openai.NewProvider(apiKey, baseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: anthropic, openai)", providerName)
	}
}

func resolveWorkspace() (*workspace.Workspace, error) {
	m, err := workspace.NewManager()
	if err != nil {
		return nil, fmt.Errorf("create workspace manager: %w", err)
	}
	return m.ResolveByNameOrDiscover("", ".")
}

func initAgent(cfg *cobot.Config, requireProvider bool) (*agent.Agent, func(), error) {
	ws, err := resolveWorkspace()
	if err != nil {
		return nil, nil, err
	}
	if err := ws.EnsureDirs(); err != nil {
		return nil, nil, fmt.Errorf("ensure workspace dirs: %w", err)
	}

	a := agent.New(cfg)

	provider, err := initProvider(cfg)
	if err != nil {
		if requireProvider {
			return nil, nil, err
		}
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	} else {
		a.SetProvider(provider)
	}

	dataDir := ws.MemoryDir()
	mc, memCleanup, err := daemon.StartOrConnect(context.Background(), dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to open memory store: %v\n", err)
	} else {
		a.SetMemoryStore(mc)
	}

	registerBuiltinTools(a, ws)

	if err := connectMCPServers(a, ws); err != nil {
		fmt.Fprintf(os.Stderr, "warning: MCP: %v\n", err)
	}

	if err := loadSkills(a, ws); err != nil {
		fmt.Fprintf(os.Stderr, "warning: skills: %v\n", err)
	}

	cleanup := func() {
		a.Close()
		if memCleanup != nil {
			memCleanup()
		}
	}
	return a, cleanup, nil
}

func registerBuiltinTools(a *agent.Agent, ws *workspace.Workspace) {
	sandbox := ws.Config.Sandbox
	sandboxChecker := &builtin.WorkspaceSandbox{
		Root:       sandbox.Root,
		AllowPaths: sandbox.AllowPaths,
	}

	a.RegisterTool(builtin.NewReadFileTool(builtin.WithReadSandbox(sandboxChecker)))
	a.RegisterTool(builtin.NewWriteFileTool(builtin.WithWriteSandbox(sandboxChecker)))
	a.RegisterTool(builtin.NewShellExecTool(builtin.WithShellSandbox(
		sandbox.Root,
		sandbox.BlockedCommands,
	)))
}

func connectMCPServers(a *agent.Agent, ws *workspace.Workspace) error {
	if len(ws.Config.EnabledMCP) == 0 {
		return nil
	}
	registry, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
	if err != nil {
		return fmt.Errorf("load MCP registry: %w", err)
	}
	mgr := mcp.NewMCPManager()
	if err := mgr.ConnectEnabled(context.Background(), registry, ws.Config.EnabledMCP); err != nil {
		return err
	}
	for _, name := range ws.Config.EnabledMCP {
		adapters, err := mgr.ToolAdapters(context.Background(), name)
		if err != nil {
			continue
		}
		for _, adapter := range adapters {
			a.RegisterTool(adapter)
		}
	}
	return nil
}

func loadSkills(a *agent.Agent, ws *workspace.Workspace) error {
	globalSkills, err := skills.LoadRegistry(xdg.SkillsRegistryDir())
	if err != nil {
		return fmt.Errorf("load global skills: %w", err)
	}
	wsSkills, err := skills.LoadRegistry(ws.SkillsDir())
	if err != nil {
		return fmt.Errorf("load workspace skills: %w", err)
	}

	all := make(map[string]*skills.Skill)
	for name, s := range globalSkills {
		all[name] = s
	}
	for name, s := range wsSkills {
		all[name] = s
	}

	for _, s := range all {
		a.RegisterTool(&skillTool{skill: s})
	}
	return nil
}

type skillTool struct {
	skill *skills.Skill
}

func (t *skillTool) Name() string        { return "skill_" + t.skill.Name }
func (t *skillTool) Description() string { return t.skill.Description }
func (t *skillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`)
}
func (t *skillTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Input string `json:"input"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if t.skill.Content != "" {
		return t.skill.Content, nil
	}
	return fmt.Sprintf("skill %q triggered with input: %s", t.skill.Name, a.Input), nil
}
