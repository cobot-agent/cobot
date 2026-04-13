package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/config"
	"github.com/cobot-agent/cobot/internal/debug"
	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
	cobot "github.com/cobot-agent/cobot/pkg"
)

var (
	cfgPath       string
	dataPath      string
	workspacePath string
	modelName     string
	debugMode     bool
)

var rootCmd = &cobra.Command{
	Use:     "cobot",
	Short:   "A personal AI agent system",
	Long:    "Cobot is a Go-based personal agent system with memory, tools, and protocols.",
	Version: "0.1.0",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debugMode {
			debug.Enable()
			debug.Log("init", "debug logging enabled")
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return tuiCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file path (default: $COBOT_CONFIG_PATH/config.yaml or $XDG_CONFIG_HOME/cobot/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataPath, "data", "", "data directory (default: $COBOT_DATA_PATH or ~/.local/share/cobot)")
	rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", "", "workspace name or directory")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "LLM model (e.g. openai:gpt-4o)")
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "enable debug logging")
}

func loadConfig() (*cobot.Config, error) {
	if dataPath != "" {
		os.Setenv("COBOT_DATA_PATH", dataPath)
	}

	if cfgPath != "" {
		os.Setenv("COBOT_CONFIG_PATH", filepath.Dir(cfgPath))
	}

	cfg := cobot.DefaultConfig()

	if cfgPath != "" {
		debug.Log("config", "loading from flag", "path", cfgPath)
		if _, err := os.Stat(cfgPath); err == nil {
			if err := config.LoadFromFile(cfg, cfgPath); err != nil {
				return nil, fmt.Errorf("load config: %w", err)
			}
		}
	} else {
		globalCfg := xdg.GlobalConfigPath()
		debug.Log("config", "loading from global", "path", globalCfg)
		if _, err := os.Stat(globalCfg); err == nil {
			if err := config.LoadFromFile(cfg, globalCfg); err != nil {
				return nil, fmt.Errorf("load global config: %w", err)
			}
		}
	}

	// Workspace resolution priority: CLI flag > env var > project discovery > default.
	// Apply env vars first so COBOT_WORKSPACE is available, then resolve workspace
	// using the correct priority.
	config.ApplyEnvVars(cfg)

	// Determine workspace name: CLI flag takes highest priority, then env var
	// (already applied to cfg.Workspace by ApplyEnvVars), then discovery/default.
	wsName := workspacePath
	if wsName == "" {
		wsName = cfg.Workspace
	}

	m, err := workspace.NewManager()
	if err == nil {
		ws, err := m.ResolveByNameOrDiscover(wsName, ".")
		if err == nil {
			cfg.Workspace = ws.Definition.Root
			if ws.Definition.Root != "" {
				if err := config.LoadWorkspaceConfig(cfg, ws.Definition.Root); err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
				}
			}
		}
	}

	if modelName != "" {
		cfg.Model = modelName
	}

	debug.Config("model", cfg.Model)
	debug.Config("max_turns", fmt.Sprintf("%d", cfg.MaxTurns))

	return cfg, nil
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
