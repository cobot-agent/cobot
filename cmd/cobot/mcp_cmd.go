package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/mcp"
	"github.com/cobot-agent/cobot/internal/xdg"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP server registry",
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
		if err != nil {
			return err
		}
		if len(registry) == 0 {
			fmt.Println("No MCP servers registered.")
			return nil
		}
		for name, entry := range registry {
			fmt.Printf("%s\t%s\t%s\n", name, entry.Transport, entry.Description)
		}
		return nil
	},
}

var mcpAddCmd = &cobra.Command{
	Use:   "add <name> -f <file>",
	Short: "Register an MCP server from a YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			return fmt.Errorf("required flag: --file")
		}
		destDir := xdg.MCPRegistryDir()
		os.MkdirAll(destDir, 0755)
		dest := filepath.Join(destDir, args[0]+".yaml")
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	},
}

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := filepath.Join(xdg.MCPRegistryDir(), args[0]+".yaml")
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove MCP server %q: %w", args[0], err)
		}
		fmt.Printf("Removed MCP server %q\n", args[0])
		return nil
	},
}

var mcpShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show MCP server details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := filepath.Join(xdg.MCPRegistryDir(), args[0]+".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("MCP server %q not found", args[0])
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	mcpAddCmd.Flags().StringP("file", "f", "", "YAML file to register")
	mcpCmd.AddCommand(mcpListCmd, mcpAddCmd, mcpRemoveCmd, mcpShowCmd)
	rootCmd.AddCommand(mcpCmd)
}
