package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/xdg"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose configuration issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		ok := true

		fmt.Println("Cobot Doctor")
		fmt.Println("============")

		configDir := xdg.CobotConfigDir()
		configPath := filepath.Join(configDir, "config.yaml")
		dataDir := xdg.CobotDataDir()

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

		fmt.Printf("\nData directory: %s\n", dataDir)
		if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
			fmt.Println("  [OK] Directory exists")
		} else {
			fmt.Println("  [MISSING] Will be created on first use")
		}

		fmt.Println()
		if ok {
			fmt.Println("All checks passed!")
		} else {
			fmt.Println("Some issues found. Run 'cobot setup' to fix.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
