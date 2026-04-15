// Package bootstrap wires together the agent runtime: workspace resolution,
// provider registry, tool registration, memory store, and sandbox configuration.
// It is the single composition root that cmd/ packages call instead of doing
// ad-hoc assembly themselves.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/config"
	"github.com/cobot-agent/cobot/internal/llm"
	"github.com/cobot-agent/cobot/internal/memory"
	"github.com/cobot-agent/cobot/internal/tools"
	"github.com/cobot-agent/cobot/internal/workspace"
	cobot "github.com/cobot-agent/cobot/pkg"
)

// Result bundles everything InitAgent produces so callers don't juggle
// multiple return values.
type Result struct {
	Agent     *agent.Agent
	Workspace *workspace.Workspace
	Cleanup   func()
}

// InitAgent creates a fully-wired Agent for the given Config. When
// requireProvider is true an error is returned if the LLM provider cannot
// be initialised (CLI chat mode); when false a warning is printed instead
// (TUI mode where the user can switch models later).
func InitAgent(cfg *cobot.Config, requireProvider bool) (*Result, error) {
	wsMgr, err := workspace.NewManager()
	if err != nil {
		return nil, fmt.Errorf("create workspace manager: %w", err)
	}

	ws, err := wsMgr.ResolveByNameOrDiscover(cfg.Workspace, ".")
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	if err := ws.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("ensure workspace dirs: %w", err)
	}

	agentCfg, _ := resolveAgentConfig(ws)
	if agentCfg != nil && agentCfg.Model != "" {
		cfg.Model = agentCfg.Model
	}
	if agentCfg != nil && agentCfg.MaxTurns > 0 {
		cfg.MaxTurns = agentCfg.MaxTurns
	}

	// Create tool registry externally and inject it into the agent.
	toolReg := tools.NewRegistry()
	a := agent.New(cfg, toolReg)

	if agentCfg != nil && agentCfg.SystemPrompt != "" {
		prompt := resolveSystemPrompt(agentCfg.SystemPrompt, ws)
		a.SetSystemPrompt(prompt)
	}

	// Create LLM registry for multi-provider model switching.
	registry := llm.NewRegistry(cfg)
	a.SetRegistry(registry)

	// SetModel resolves the "provider:model" spec and initializes the provider.
	if err := a.SetModel(cfg.Model); err != nil {
		if requireProvider {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	if err := ConfigureAgentForWorkspace(a, ws, registry); err != nil {
		return nil, err
	}

	// a.Close() already closes the memory store; no need for separate cleanup.
	cleanup := func() { a.Close() }
	return &Result{Agent: a, Workspace: ws, Cleanup: cleanup}, nil
}

// ConfigureAgentForWorkspace (re)configures an existing agent for a workspace:
// memory store, sandbox-scoped tools, workspace tools, and delegate tool.
// It is called once during InitAgent and again when the TUI switches workspaces.
func ConfigureAgentForWorkspace(a *agent.Agent, ws *workspace.Workspace, registry cobot.ModelResolver) error {
	agentCfg, _ := resolveAgentConfig(ws)

	if agentCfg != nil && agentCfg.SystemPrompt != "" {
		prompt := resolveSystemPrompt(agentCfg.SystemPrompt, ws)
		a.SetSystemPrompt(prompt)
	}

	// --- memory ---
	if old := a.MemoryStore(); old != nil {
		if err := old.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close memory store: %v\n", err)
		}
	}
	dataDir := ws.MemoryDir()
	store, err := memory.OpenStore(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to open memory store: %v\n", err)
	} else {
		a.SetMemoryStore(store)
		a.SetMemoryRecall(store)
		a.RegisterTool(memory.NewMemorySearchTool(store))
		a.RegisterTool(memory.NewMemoryStoreTool(store))
		a.RegisterTool(memory.NewL3DeepSearchTool(store))
	}

	// --- sandbox ---
	sandboxRoot := resolveSandboxRoot(ws)
	sandboxCfg := ws.Config.Sandbox
	if agentCfg != nil && agentCfg.Sandbox != nil {
		if agentCfg.Sandbox.Root != "" {
			sandboxCfg.Root = agentCfg.Sandbox.Root
			sandboxRoot = agentCfg.Sandbox.Root
		}
		if len(agentCfg.Sandbox.AllowPaths) > 0 {
			sandboxCfg.AllowPaths = agentCfg.Sandbox.AllowPaths
		}
		if len(agentCfg.Sandbox.BlockedCommands) > 0 {
			sandboxCfg.BlockedCommands = agentCfg.Sandbox.BlockedCommands
		}
	}
	sandbox := &cobot.SandboxConfig{
		Root:          sandboxCfg.Root,
		AllowPaths:    sandboxCfg.AllowPaths,
		ReadonlyPaths: sandboxCfg.ReadonlyPaths,
	}
	a.RegisterTool(tools.NewReadFileTool(tools.WithReadSandbox(sandbox)))
	a.RegisterTool(tools.NewWriteFileTool(tools.WithWriteSandbox(sandbox)))
	a.RegisterTool(tools.NewShellExecTool(
		tools.WithShellWorkdir(sandboxRoot),
		tools.WithShellSandboxConfig(&cobot.SandboxConfig{
			BlockedCommands: sandboxCfg.BlockedCommands,
			AllowNetwork:    sandboxCfg.AllowNetwork,
		}),
	))

	// --- workspace tools ---
	tools.RegisterWorkspaceTools(a.ToolRegistry(), ws)

	// --- delegate tool ---
	a.RegisterTool(tools.NewDelegateTool(func() cobot.SubAgent {
		sub := agent.New(a.Config(), a.ToolRegistry().Clone())
		sub.SetProvider(a.Provider())
		sub.SetRegistry(registry)
		return sub
	}))

	return nil
}

// --- private helpers (moved from cmd/cobot/helpers.go) ---

func resolveSandboxRoot(ws *workspace.Workspace) string {
	if ws.Config.Sandbox.Root != "" {
		return ws.Config.Sandbox.Root
	}
	if ws.Definition.Root != "" {
		return ws.Definition.Root
	}
	return ws.DataDir
}

func resolveSystemPrompt(value string, ws *workspace.Workspace) string {
	if strings.HasSuffix(value, ".md") {
		path := filepath.Join(ws.DataDir, value)
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return value
}

func resolveAgentConfig(ws *workspace.Workspace) (*config.AgentConfig, error) {
	configs, err := config.LoadAgentConfigs(ws.AgentsDir())
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
