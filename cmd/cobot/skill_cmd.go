package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/skills"
	"github.com/cobot-agent/cobot/internal/workspace"
	"github.com/cobot-agent/cobot/internal/xdg"
)

var skillScope string

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills",
}

var skillListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List skills",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, _ := cmd.Flags().GetString("scope")

		if scope == "" || scope == "global" {
			globalDir := xdg.SkillsRegistryDir()
			globalSkills, err := skills.LoadRegistry(globalDir)
			if err != nil {
				return fmt.Errorf("load global skills: %w", err)
			}
			if len(globalSkills) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Global Skills:")
				for name, s := range globalSkills {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", name, s.Format)
					if s.Description != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", s.Description)
					}
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "No global skills found.")
			}
		}

		if scope == "" || scope == "workspace" {
			m, err := workspace.NewManager()
			if err != nil {
				if scope == "workspace" {
					return fmt.Errorf("create workspace manager: %w", err)
				}
				return nil
			}
			ws, err := m.ResolveByNameOrDiscover("", ".")
			if err != nil {
				if scope == "workspace" {
					return fmt.Errorf("resolve workspace: %w", err)
				}
				return nil
			}
			wsSkills, err := skills.LoadRegistry(ws.SkillsDir())
			if err != nil {
				if scope == "workspace" {
					return fmt.Errorf("load workspace skills: %w", err)
				}
				return nil
			}
			if len(wsSkills) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nWorkspace Skills (%s):\n", ws.Config.Name)
				for name, s := range wsSkills {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", name, s.Format)
					if s.Description != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", s.Description)
					}
				}
			} else if scope == "workspace" {
				fmt.Fprintln(cmd.OutOrStdout(), "\nNo workspace skills found.")
			}
		}

		return nil
	},
}

var skillAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a skill to the global registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file is required")
		}

		name := args[0]
		ext := strings.ToLower(filepath.Ext(filePath))

		regDir := xdg.SkillsRegistryDir()
		if err := os.MkdirAll(regDir, 0755); err != nil {
			return fmt.Errorf("create skills directory: %w", err)
		}

		src, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open source file: %w", err)
		}
		defer src.Close()

		dstPath := filepath.Join(regDir, name+ext)
		dst, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("create destination file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("copy file: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Added skill '%s' to global registry\n", name)
		return nil
	},
}

var skillRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a skill from the global registry",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		regDir := xdg.SkillsRegistryDir()

		candidates := []string{
			filepath.Join(regDir, name+".yaml"),
			filepath.Join(regDir, name+".yml"),
			filepath.Join(regDir, name+".md"),
		}

		removed := false
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				if err := os.Remove(c); err != nil {
					return fmt.Errorf("remove %s: %w", c, err)
				}
				removed = true
				break
			}
		}

		if !removed {
			dirCandidate := filepath.Join(regDir, name)
			if info, err := os.Stat(dirCandidate); err == nil && info.IsDir() {
				if err := os.RemoveAll(dirCandidate); err != nil {
					return fmt.Errorf("remove directory: %w", err)
				}
				removed = true
			}
		}

		if !removed {
			return fmt.Errorf("skill '%s' not found in global registry", name)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed skill '%s' from global registry\n", name)
		return nil
	},
}

func init() {
	skillListCmd.Flags().String("scope", "", "Scope to list: global, workspace (default: both)")
	skillAddCmd.Flags().StringP("file", "f", "", "Skill file to add")

	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillRemoveCmd)
	rootCmd.AddCommand(skillCmd)
}
