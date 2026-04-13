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

func getPersonaService() (*persona.Service, error) {
	m, err := workspace.NewManager()
	if err != nil {
		return nil, fmt.Errorf("create workspace manager: %w", err)
	}
	ws := m.Current()
	return persona.NewService(ws), nil
}

var personaInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize persona files",
	Long:  "Create SOUL.md, USER.md, and MEMORY.md in the current workspace if they don't exist.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := getPersonaService()
		if err != nil {
			return err
		}

		if err := svc.EnsureFiles(); err != nil {
			return fmt.Errorf("ensure persona files: %w", err)
		}

		fmt.Println("Persona files initialized:")
		fmt.Printf("  SOUL:   %s\n", svc.GetSoulPath())
		fmt.Printf("  USER:   %s\n", svc.GetUserPath())
		fmt.Printf("  MEMORY: %s\n", svc.GetMemoryPath())
		return nil
	},
}

var personaEditCmd = &cobra.Command{
	Use:   "edit [soul|user|memory]",
	Short: "Edit a persona file",
	Long:  "Open a persona file (SOUL.md, USER.md, or MEMORY.md) in your default editor.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := getPersonaService()
		if err != nil {
			return err
		}

		svc.EnsureFiles()

		var path string
		switch args[0] {
		case "soul":
			path = svc.GetSoulPath()
		case "user":
			path = svc.GetUserPath()
		case "memory":
			path = svc.GetMemoryPath()
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
		svc, err := getPersonaService()
		if err != nil {
			return err
		}

		var content string
		switch args[0] {
		case "soul":
			content, err = svc.LoadSoul()
		case "user":
			content, err = svc.LoadUser()
		case "memory":
			content, err = svc.LoadMemory()
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
