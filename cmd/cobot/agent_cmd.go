package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/config"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List agents in current workspace",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := resolveWorkspace()
		if err != nil {
			return err
		}

		configs, err := config.LoadAgentConfigs(ws.AgentsDir())
		if err != nil {
			return err
		}

		if len(configs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No agents found.")
			return nil
		}

		defaultAgent := ws.Config.DefaultAgent
		if defaultAgent == "" {
			defaultAgent = "main"
		}

		names := make([]string, 0, len(configs))
		for name := range configs {
			names = append(names, name)
		}
		sort.Strings(names)

		fmt.Fprintln(cmd.OutOrStdout(), "Agents:")
		for _, name := range names {
			cfg := configs[name]
			marker := " "
			if name == defaultAgent {
				marker = "*"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s %s  %s\n", marker, name, cfg.Model)
		}
		return nil
	},
}

var agentShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show agent configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := resolveWorkspace()
		if err != nil {
			return err
		}

		configs, err := config.LoadAgentConfigs(ws.AgentsDir())
		if err != nil {
			return err
		}

		name := args[0]
		cfg, ok := configs[name]
		if !ok {
			return fmt.Errorf("agent '%s' not found", name)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Name:          %s\n", cfg.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "Model:         %s\n", cfg.Model)
		if cfg.SystemPrompt != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "System Prompt: %s\n", cfg.SystemPrompt)
		}
		if len(cfg.EnabledMCP) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled MCP:   %v\n", cfg.EnabledMCP)
		}
		if len(cfg.EnabledSkills) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled Skills: %v\n", cfg.EnabledSkills)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Max Turns:     %d\n", cfg.MaxTurns)
		return nil
	},
}

func init() {
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
	rootCmd.AddCommand(agentCmd)
}
