package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List and manage tools",
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		builtinTools := []string{
			"filesystem_read", "filesystem_write", "shell_exec",
			"memory_search", "memory_store", "subagent_spawn",
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Built-in tools (%d):\n", len(builtinTools))
		for _, name := range builtinTools {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
		}

		if cfg.Tools.MCPServers != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\nMCP servers (%d):\n", len(cfg.Tools.MCPServers))
			for name, srv := range cfg.Tools.MCPServers {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", name, srv.Transport)
			}
		}

		return nil
	},
}

func init() {
	toolsCmd.AddCommand(toolsListCmd)
	rootCmd.AddCommand(toolsCmd)
}
