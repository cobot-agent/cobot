package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/workspace"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

var workspaceInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .cobot workspace in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if workspacePath != "" {
			dir = workspacePath
		}
		ws, err := workspace.Init(dir)
		if err != nil {
			return err
		}
		fmt.Printf("Initialized cobot workspace at %s\n", ws.Root)
		return nil
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceInitCmd)
	rootCmd.AddCommand(workspaceCmd)
}
