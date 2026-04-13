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

type SkillTool struct {
	skill *skills.Skill
}

func (t *SkillTool) Name() string        { return "skill_" + t.skill.Name }
func (t *SkillTool) Description() string { return t.skill.Description }
func (t *SkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`)
}

func (t *SkillTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.skill.Content != "" {
		return t.skill.Content, nil
	}
	return fmt.Sprintf("Skill %q triggered", t.skill.Name), nil
}

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

func resolveSandboxRoot(ws *workspace.Workspace) string {
	if ws.Config.Sandbox.Root != "" {
		return ws.Config.Sandbox.Root
	}
	if ws.Definition.Root != "" {
		return ws.Definition.Root
	}
	return ws.DataDir
}

func resolveAgentConfig(ws *workspace.Workspace) (*agent.AgentConfig, error) {
	configs, err := agent.LoadAgentConfigs(ws.AgentsDir())
	if err != nil {
		return nil, nil
	}

	name := ws.Config.DefaultAgent
	if name == "" {
		name = "main"
	}

	if cfg, ok := configs[name]; ok {
		return cfg, nil
	}
	return nil, nil
}

func connectMCPServers(a *agent.Agent, ws *workspace.Workspace) error {
	if len(ws.Config.EnabledMCP) == 0 {
		return nil
	}
	registry, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
	if err != nil {
		return err
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

func loadAndRegisterSkills(a *agent.Agent, ws *workspace.Workspace, agentCfg *agent.AgentConfig) {
	globalSkills, _ := skills.LoadRegistry(xdg.SkillsRegistryDir())
	wsSkills, _ := skills.LoadRegistry(ws.SkillsDir())

	all := make(map[string]*skills.Skill)
	for name, s := range globalSkills {
		all[name] = s
	}
	for name, s := range wsSkills {
		all[name] = s
	}

	var enabled []string
	if agentCfg != nil && len(agentCfg.EnabledSkills) > 0 {
		enabled = agentCfg.EnabledSkills
	} else if len(ws.Config.EnabledSkills) > 0 {
		enabled = ws.Config.EnabledSkills
	}

	enabledSet := make(map[string]bool, len(enabled))
	for _, s := range enabled {
		enabledSet[s] = true
	}

	for name, s := range all {
		if len(enabled) > 0 && !enabledSet[name] {
			continue
		}
		a.RegisterTool(&SkillTool{skill: s})
	}
}

func initAgent(cfg *cobot.Config, requireProvider bool) (*agent.Agent, *workspace.Workspace, func(), error) {
	wsMgr, err := workspace.NewManager()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create workspace manager: %w", err)
	}

	ws, err := wsMgr.ResolveByNameOrDiscover(cfg.Workspace, ".")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve workspace: %w", err)
	}
	if err := ws.EnsureDirs(); err != nil {
		return nil, nil, nil, fmt.Errorf("ensure workspace dirs: %w", err)
	}

	agentCfg, _ := resolveAgentConfig(ws)
	if agentCfg != nil && agentCfg.Model != "" {
		cfg.Model = agentCfg.Model
	}
	if agentCfg != nil && agentCfg.MaxTurns > 0 {
		cfg.MaxTurns = agentCfg.MaxTurns
	}

	a := agent.New(cfg)

	provider, err := initProvider(cfg)
	if err != nil {
		if requireProvider {
			return nil, nil, nil, err
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

	sandboxRoot := resolveSandboxRoot(ws)
	sandbox := &builtin.WorkspaceSandbox{
		Root:          ws.Config.Sandbox.Root,
		AllowPaths:    ws.Config.Sandbox.AllowPaths,
		ReadonlyPaths: ws.Config.Sandbox.ReadonlyPaths,
	}
	a.RegisterTool(builtin.NewReadFileTool(builtin.WithReadSandbox(sandbox)))
	a.RegisterTool(builtin.NewWriteFileTool(builtin.WithWriteSandbox(sandbox)))
	a.RegisterTool(builtin.NewShellExecTool(
		builtin.WithShellWorkdir(sandboxRoot),
		builtin.WithShellBlockedCommands(ws.Config.Sandbox.BlockedCommands),
	))

	if err := connectMCPServers(a, ws); err != nil {
		fmt.Fprintf(os.Stderr, "warning: MCP: %v\n", err)
	}

	loadAndRegisterSkills(a, ws, agentCfg)

	cleanup := func() {
		a.Close()
		if memCleanup != nil {
			memCleanup()
		}
	}
	return a, ws, cleanup, nil
}
