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

		workspaces, err := manager.List()
		if err != nil {
			return err
		}

		current := manager.Current()

		fmt.Fprintln(cmd.OutOrStdout(), "Workspaces:")
		for _, ws := range workspaces {
			marker := " "
			if ws.ID == current.ID {
				marker = "*"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s %s (%s) - %s\n", marker, ws.Name, ws.Type, ws.ID[:8])
			if ws.Root != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Root: %s\n", ws.Root)
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
		ws, err := manager.Create(name, workspace.WorkspaceTypeCustom, "")
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created workspace '%s' (%s)\n", ws.Name, ws.ID[:8])
		fmt.Fprintf(cmd.OutOrStdout(), "  Config: %s\n", ws.ConfigDir)
		fmt.Fprintf(cmd.OutOrStdout(), "  Data:   %s\n", ws.DataDir)
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

		// 转换为绝对路径
		absPath, err := filepath.Abs(projectDir)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		// 确保 .cobot 目录存在
		cobotDir := filepath.Join(absPath, ".cobot")
		if err := os.MkdirAll(cobotDir, 0755); err != nil {
			return fmt.Errorf("create .cobot directory: %w", err)
		}

		ws, err := manager.CreateProject(absPath)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created project workspace '%s' (%s)\n", ws.Name, ws.ID[:8])
		fmt.Fprintf(cmd.OutOrStdout(), "  Root:   %s\n", ws.Root)
		fmt.Fprintf(cmd.OutOrStdout(), "  Config: %s\n", ws.ConfigDir)
		fmt.Fprintf(cmd.OutOrStdout(), "  Data:   %s\n", ws.DataDir)
		return nil
	},
}

var workspaceSwitchCmd = &cobra.Command{
	Use:   "switch <name-or-id>",
	Short: "Switch to a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		ws, err := manager.Switch(args[0])
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Switched to workspace '%s' (%s)\n", ws.Name, ws.ID[:8])
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
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

var workspaceCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		ws := manager.Current()
		fmt.Fprintf(cmd.OutOrStdout(), "Current workspace: %s (%s)\n", ws.Name, ws.ID[:8])
		fmt.Fprintf(cmd.OutOrStdout(), "  Type:   %s\n", ws.Type)
		if ws.Root != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Root:   %s\n", ws.Root)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Config: %s\n", ws.ConfigDir)
		fmt.Fprintf(cmd.OutOrStdout(), "  Data:   %s\n", ws.DataDir)
		return nil
	},
}

var workspaceRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a workspace",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := workspace.NewManager()
		if err != nil {
			return err
		}

		if err := manager.Rename(args[0], args[1]); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Renamed workspace '%s' to '%s'\n", args[0], args[1])
		return nil
	},
}

func init() {
	workspaceDeleteCmd.Flags().Bool("force", false, "Force deletion without confirmation")

	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceCreateCmd)
	workspaceCmd.AddCommand(workspaceProjectCmd)
	workspaceCmd.AddCommand(workspaceSwitchCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
	workspaceCmd.AddCommand(workspaceCurrentCmd)
	workspaceCmd.AddCommand(workspaceRenameCmd)
	rootCmd.AddCommand(workspaceCmd)
}
