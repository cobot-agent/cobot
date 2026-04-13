package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/cobot-agent/cobot/internal/mcp"
	"github.com/cobot-agent/cobot/internal/xdg"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers",
}

var mcpListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List registered MCP servers",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No MCP servers registered.")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "MCP Servers:")
		for name, entry := range entries {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", name, entry.Transport)
			if entry.Command != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    Command: %s\n", entry.Command)
			}
			if entry.URL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    URL: %s\n", entry.URL)
			}
			if entry.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    Description: %s\n", entry.Description)
			}
		}
		return nil
	},
}

var mcpAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register an MCP server from a YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file is required")
		}

		name := args[0]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		var entry mcp.RegistryEntry
		if err := yaml.Unmarshal(data, &entry); err != nil {
			return fmt.Errorf("parse YAML: %w", err)
		}

		if entry.Name == "" {
			entry.Name = name
		}

		outData, err := yaml.Marshal(&entry)
		if err != nil {
			return fmt.Errorf("marshal YAML: %w", err)
		}

		regDir := xdg.MCPRegistryDir()
		if err := os.MkdirAll(regDir, 0755); err != nil {
			return fmt.Errorf("create registry directory: %w", err)
		}

		outPath := filepath.Join(regDir, name+".yaml")
		if err := os.WriteFile(outPath, outData, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Registered MCP server '%s'\n", name)
		return nil
	},
}

var mcpRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Unregister an MCP server",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		regDir := xdg.MCPRegistryDir()
		yamlPath := filepath.Join(regDir, name+".yaml")

		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			return fmt.Errorf("MCP server '%s' not found", name)
		}

		if err := os.Remove(yamlPath); err != nil {
			return fmt.Errorf("remove: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed MCP server '%s'\n", name)
		return nil
	},
}

var mcpShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show MCP server details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		entries, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
		if err != nil {
			return err
		}

		entry, ok := entries[name]
		if !ok {
			return fmt.Errorf("MCP server '%s' not found", name)
		}

		out, err := yaml.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		fmt.Fprint(cmd.OutOrStdout(), string(out))
		return nil
	},
}

var mcpTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Test connection to an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		entries, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
		if err != nil {
			return err
		}

		entry, ok := entries[name]
		if !ok {
			return fmt.Errorf("MCP server '%s' not found", name)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		mgr := mcp.NewMCPManager()
		defer mgr.Close()

		if err := mgr.ConnectFromRegistry(ctx, name, entry); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Connection failed: %v\n", err)
			return nil
		}
		defer mgr.Disconnect(ctx, name)

		tools, err := mgr.ListTools(ctx, name)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Connected but failed to list tools: %v\n", err)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Connection successful! %d tools available.\n", len(tools))
		for _, tool := range tools {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", tool.Name)
		}
		return nil
	},
}

func init() {
	mcpAddCmd.Flags().StringP("file", "f", "", "YAML file to register")

	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
	mcpCmd.AddCommand(mcpShowCmd)
	mcpCmd.AddCommand(mcpTestCmd)
	rootCmd.AddCommand(mcpCmd)
}
