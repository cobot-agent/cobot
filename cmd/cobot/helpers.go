package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/llm/anthropic"
	"github.com/cobot-agent/cobot/internal/llm/openai"
	"github.com/cobot-agent/cobot/internal/memory/daemon"
	"github.com/cobot-agent/cobot/internal/tools/builtin"
	"github.com/cobot-agent/cobot/internal/workspace"
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

	a.RegisterTool(builtin.NewReadFileTool())
	a.RegisterTool(builtin.NewWriteFileTool())
	a.RegisterTool(builtin.NewShellExecTool())

	cleanup := func() {
		a.Close()
		if memCleanup != nil {
			memCleanup()
		}
	}
	return a, cleanup, nil
}
