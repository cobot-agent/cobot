package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Manage scheduled tasks",
}

var schedulerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "No scheduler running. Start via config or TUI.")
		return nil
	},
}

func init() {
	schedulerCmd.AddCommand(schedulerListCmd)
	rootCmd.AddCommand(schedulerCmd)
}
