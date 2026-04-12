package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/skills"
	"github.com/cobot-agent/cobot/internal/xdg"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills",
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, _ := cmd.Flags().GetString("scope")
		var dir string
		if scope == "workspace" {
			ws, err := resolveWorkspace()
			if err != nil {
				return err
			}
			dir = ws.SkillsDir()
		} else {
			dir = xdg.SkillsRegistryDir()
		}
		reg, err := skills.LoadRegistry(dir)
		if err != nil {
			return err
		}
		if len(reg) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No skills found.")
			return nil
		}
		for name, s := range reg {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", name, s.Format, s.Description)
		}
		return nil
	},
}

var skillAddCmd = &cobra.Command{
	Use:   "add <name> -f <file>",
	Short: "Add a skill to the global registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			return fmt.Errorf("required flag: --file")
		}
		ext := filepath.Ext(file)
		destDir := xdg.SkillsRegistryDir()
		os.MkdirAll(destDir, 0755)
		dest := filepath.Join(destDir, args[0]+ext)
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	},
}

var skillRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a skill from the global registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := xdg.SkillsRegistryDir()
		for _, ext := range []string{".yaml", ".md"} {
			path := filepath.Join(dir, args[0]+ext)
			if _, err := os.Stat(path); err == nil {
				return os.Remove(path)
			}
		}
		dirPath := filepath.Join(dir, args[0])
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			return os.RemoveAll(dirPath)
		}
		return fmt.Errorf("skill %q not found", args[0])
	},
}

func init() {
	skillListCmd.Flags().String("scope", "global", "List skills from: global|workspace")
	skillAddCmd.Flags().StringP("file", "f", "", "Skill file to add")
	skillCmd.AddCommand(skillListCmd, skillAddCmd, skillRemoveCmd)
	rootCmd.AddCommand(skillCmd)
}
