package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/config"
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
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", "", "workspace directory")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "LLM model (e.g. openai:gpt-4o)")
}

func loadConfig() (*cobot.Config, error) {
	cfg := cobot.DefaultConfig()
	if cfgPath != "" {
		if err := config.LoadFromFile(cfg, cfgPath); err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	}
	if modelName != "" {
		cfg.Model = modelName
	}
	if workspacePath != "" {
		cfg.Workspace = workspacePath
	}
	return cfg, nil
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
