package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/workspace"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

var workspaceListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all workspaces",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		defs, err := manager.List()
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Workspaces:")
		for _, def := range defs {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", def.Name, def.Type)
			if def.Root != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Root: %s\n", def.Root)
			}
			if def.Path != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Path: %s\n", def.Path)
			}
		}
		return nil
	},
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new custom workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		name := args[0]
		root, _ := cmd.Flags().GetString("root")
		customPath, _ := cmd.Flags().GetString("path")
		ws, err := manager.Create(name, workspace.WorkspaceTypeCustom, root, customPath)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created workspace '%s' (%s)\n", ws.Config.Name, ws.Config.ID[:8])
		fmt.Fprintf(cmd.OutOrStdout(), "  Data: %s\n", ws.DataDir)
		return nil
	},
}

var workspaceProjectCmd = &cobra.Command{
	Use:   "project [path]",
	Short: "Create a workspace from a project directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		projectDir := "."
		if len(args) > 0 {
			projectDir = args[0]
		}

		absPath, err := filepath.Abs(projectDir)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		cobotDir := filepath.Join(absPath, ".cobot")
		if err := os.MkdirAll(cobotDir, 0755); err != nil {
			return fmt.Errorf("create .cobot directory: %w", err)
		}

		ws, err := manager.Create(filepath.Base(absPath), workspace.WorkspaceTypeProject, absPath, "")
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created project workspace '%s' (%s)\n", ws.Config.Name, ws.Config.ID[:8])
		fmt.Fprintf(cmd.OutOrStdout(), "  Root: %s\n", ws.Definition.Root)
		fmt.Fprintf(cmd.OutOrStdout(), "  Data: %s\n", ws.DataDir)
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			return fmt.Errorf("use --force to confirm deletion of workspace '%s'", args[0])
		}

		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		if err := manager.Delete(args[0]); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Deleted workspace '%s'\n", args[0])
		return nil
	},
}

var workspaceShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show workspace details",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		ws, err := manager.ResolveByNameOrDiscover(name, ".")
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s (%s)\n", ws.Config.Name, ws.Config.ID[:8])
		fmt.Fprintf(cmd.OutOrStdout(), "  Type:   %s\n", ws.Definition.Type)
		if ws.Definition.Root != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Root:   %s\n", ws.Definition.Root)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Data:   %s\n", ws.DataDir)
		return nil
	},
}

func init() {
	workspaceDeleteCmd.Flags().Bool("force", false, "Force deletion without confirmation")
	workspaceCreateCmd.Flags().String("root", "", "Project root directory")
	workspaceCreateCmd.Flags().String("path", "", "Custom data directory path")

	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceCreateCmd)
	workspaceCmd.AddCommand(workspaceProjectCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
	workspaceCmd.AddCommand(workspaceShowCmd)
	rootCmd.AddCommand(workspaceCmd)
}
