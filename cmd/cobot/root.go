package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/config"
	"github.com/cobot-agent/cobot/internal/debug"
	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
	cobot "github.com/cobot-agent/cobot/pkg"
)

var (
	cfgPath       string
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
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file path (default: $XDG_CONFIG_HOME/cobot/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", "", "workspace directory")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "LLM model (e.g. openai:gpt-4o)")
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "enable debug logging")
}

func loadConfig() (*cobot.Config, error) {
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

	if workspacePath != "" {
		cfg.Workspace = workspacePath
		if err := config.LoadWorkspaceConfig(cfg, workspacePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	} else {
		m, err := workspace.NewManager()
		if err == nil {
			ws, err := workspace.DiscoverOrCurrent(".", m)
			if err == nil {
				cfg.Workspace = ws.Root
				if err := config.LoadWorkspaceConfig(cfg, ws.Root); err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
				}
			}
		}
	}

	config.ApplyEnvVars(cfg)

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
