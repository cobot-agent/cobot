package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "List or set the active model",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available models:")
		fmt.Println("  openai:gpt-4o")
		fmt.Println("  openai:gpt-4o-mini")
		fmt.Println("  openrouter:auto")
		return nil
	},
}

func init() {
	modelCmd.AddCommand(modelListCmd)
	rootCmd.AddCommand(modelCmd)
}
