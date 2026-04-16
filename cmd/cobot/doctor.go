package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/workspace"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose configuration issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		ok := true

		fmt.Println("Cobot Personal Agent Doctor")
		fmt.Println("===========================")

		manager, err := workspace.NewManager()
		if err != nil {
			fmt.Printf("  [ERROR] Failed to create workspace manager: %v\n", err)
			return err
		}

		ws, err := manager.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		configDir := workspace.ConfigDir()
		configPath := filepath.Join(configDir, "config.yaml")
		dataDir := workspace.DataDir()

		fmt.Printf("\nConfig directory: %s\n", configDir)
		if info, err := os.Stat(configDir); err == nil && info.IsDir() {
			fmt.Println("  [OK] Directory exists")
		} else {
			fmt.Println("  [MISSING] Directory not found")
			ok = false
		}

		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("  [OK] Config file: %s\n", configPath)
			cfg, err := loadConfig()
			if err != nil {
				fmt.Printf("  [ERROR] Failed to load config: %v\n", err)
				ok = false
			} else {
				fmt.Printf("  [OK] Model: %s\n", cfg.Model)
				if len(cfg.APIKeys) > 0 {
					providers := make([]string, 0, len(cfg.APIKeys))
					for k := range cfg.APIKeys {
						providers = append(providers, k)
					}
					fmt.Printf("  [OK] API keys configured: %s\n", providers)
				} else {
					fmt.Println("  [WARN] No API keys configured")
					fmt.Println("         Run 'cobot setup' or set OPENAI_API_KEY")
					ok = false
				}
			}
		} else {
			fmt.Println("  [WARN] No config file found")
			fmt.Printf("         Run 'cobot setup' to create %s\n", configPath)
			ok = false
		}

		fmt.Println("\nCurrent workspace:")
		fmt.Printf("  Name: %s (%s)\n", ws.Config.Name, ws.Config.ID[:8])
		fmt.Printf("  Type: %s\n", ws.Definition.Type)
		if ws.Definition.Root != "" {
			fmt.Printf("  Root: %s\n", ws.Definition.Root)
		}

		fmt.Println("\nPersona files:")
		if _, err := os.Stat(ws.GetSoulPath()); err == nil {
			fmt.Printf("  [OK] SOUL:   %s\n", ws.GetSoulPath())
		} else {
			fmt.Printf("  [MISSING] SOUL:   %s\n", ws.GetSoulPath())
		}
		if _, err := os.Stat(ws.GetUserPath()); err == nil {
			fmt.Printf("  [OK] USER:   %s\n", ws.GetUserPath())
		} else {
			fmt.Printf("  [MISSING] USER:   %s\n", ws.GetUserPath())
		}
		if _, err := os.Stat(ws.GetMemoryMdPath()); err == nil {
			fmt.Printf("  [OK] MEMORY: %s\n", ws.GetMemoryMdPath())
		} else {
			fmt.Printf("  [MISSING] MEMORY: %s\n", ws.GetMemoryMdPath())
		}

		fmt.Printf("\nData directory: %s\n", dataDir)
		if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
			fmt.Println("  [OK] Directory exists")
			memDir := ws.MemoryDir()
			if info, err := os.Stat(memDir); err == nil && info.IsDir() {
				fmt.Printf("  [OK] Memory dir: %s\n", memDir)
			} else {
				fmt.Printf("  [INFO] Memory dir will be created: %s\n", memDir)
			}
			fmt.Printf("  Space:          %s\n", ws.SpaceDir())
			fmt.Printf("  MCP:            %s\n", ws.MCPDir())
			fmt.Printf("  Global Skills:  %s\n", workspace.GlobalSkillsDir())
			fmt.Printf("  Workspace Skills: %s\n", ws.SkillsDir())
		} else {
			fmt.Println("  [MISSING] Will be created on first use")
		}

		fmt.Println()
		if ok {
			fmt.Println("All critical checks passed!")
		} else {
			fmt.Println("Some issues found. Run 'cobot setup' to fix.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
