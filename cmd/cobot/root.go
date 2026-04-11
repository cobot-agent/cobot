package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/config"
	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
	cobot "github.com/cobot-agent/cobot/pkg"
)

var (
	cfgPath       string
	workspacePath string
	modelName     string
)

var rootCmd = &cobra.Command{
	Use:     "cobot",
	Short:   "A personal AI agent system",
	Long:    "Cobot is a Go-based personal agent system with memory, tools, and protocols.",
	Version: "0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tuiCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file path (default: $XDG_CONFIG_HOME/cobot/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", "", "workspace directory")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "LLM model (e.g. openai:gpt-4o)")
}

func loadConfig() (*cobot.Config, error) {
	if err := workspace.EnsureGlobalDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg := cobot.DefaultConfig()

	if cfgPath != "" {
		if _, err := os.Stat(cfgPath); err == nil {
			if err := config.LoadFromFile(cfg, cfgPath); err != nil {
				return nil, fmt.Errorf("load config: %w", err)
			}
		}
	} else {
		globalCfg := xdg.GlobalConfigPath()
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
	} else if ws, err := workspace.Discover("."); err == nil {
		cfg.Workspace = ws.Root
		if err := config.LoadWorkspaceConfig(cfg, ws.Root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	config.ApplyEnvVars(cfg)

	if modelName != "" {
		cfg.Model = modelName
	}
	return cfg, nil
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
