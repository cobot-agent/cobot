package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/persona"
	"github.com/cobot-agent/cobot/internal/workspace"
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage personal agent persona",
	Long:  "Manage SOUL.md, USER.md, and MEMORY.md files that define the agent's personality and user profile.",
}

var personaInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize persona files",
	Long:  "Create SOUL.md, USER.md, and MEMORY.md in the global config directory if they don't exist.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := workspace.EnsureGlobalWorkspace(); err != nil {
			return fmt.Errorf("ensure global workspace: %w", err)
		}

		p := persona.New()
		if err := p.EnsureFiles(); err != nil {
			return fmt.Errorf("ensure persona files: %w", err)
		}

		fmt.Println("Persona files initialized:")
		fmt.Printf("  SOUL:   %s\n", p.GetSoulPath())
		fmt.Printf("  USER:   %s\n", p.GetUserPath())
		fmt.Printf("  MEMORY: %s\n", p.GetMemoryPath())
		return nil
	},
}

var personaEditCmd = &cobra.Command{
	Use:   "edit [soul|user|memory]",
	Short: "Edit a persona file",
	Long:  "Open a persona file (SOUL.md, USER.md, or MEMORY.md) in your default editor.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := workspace.EnsureGlobalWorkspace(); err != nil {
			return fmt.Errorf("ensure global workspace: %w", err)
		}

		p := persona.New()
		p.EnsureFiles()

		var path string
		switch args[0] {
		case "soul":
			path = p.GetSoulPath()
		case "user":
			path = p.GetUserPath()
		case "memory":
			path = p.GetMemoryPath()
		default:
			return fmt.Errorf("unknown persona file: %s (valid: soul, user, memory)", args[0])
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		execCmd := exec.Command(editor, path)
		execCmd.Stdin = os.Stdin
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr

		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}

		return nil
	},
}

var personaShowCmd = &cobra.Command{
	Use:   "show [soul|user|memory]",
	Short: "Show a persona file",
	Long:  "Display the contents of a persona file (SOUL.md, USER.md, or MEMORY.md).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := persona.New()

		var content string
		var err error
		switch args[0] {
		case "soul":
			content, err = p.LoadSoul()
		case "user":
			content, err = p.LoadUser()
		case "memory":
			content, err = p.LoadMemory()
		default:
			return fmt.Errorf("unknown persona file: %s (valid: soul, user, memory)", args[0])
		}

		if err != nil {
			return fmt.Errorf("load file: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	},
}

func init() {
	personaCmd.AddCommand(personaInitCmd)
	personaCmd.AddCommand(personaEditCmd)
	personaCmd.AddCommand(personaShowCmd)
	rootCmd.AddCommand(personaCmd)
}
