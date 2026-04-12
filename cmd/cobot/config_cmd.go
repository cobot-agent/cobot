package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cobot-agent/cobot/internal/config"
	"github.com/cobot-agent/cobot/internal/workspace"
	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show full resolved config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		masked := *cfg
		if masked.APIKeys != nil {
			masked.APIKeys = make(map[string]string)
			for k := range cfg.APIKeys {
				masked.APIKeys[k] = "***"
			}
		}
		data, _ := json.MarshalIndent(masked, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a specific config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		key := args[0]
		value := getConfigValue(cfg, key)
		if value == "" {
			return fmt.Errorf("config key not found: %s", key)
		}
		fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		ws, err := workspace.Discover("")
		if err != nil {
			return fmt.Errorf("finding workspace: %w", err)
		}

		cfg := cobot.DefaultConfig()
		_ = config.LoadFromFile(cfg, ws.ConfigPath)

		if err := setConfigValue(cfg, key, value); err != nil {
			return err
		}

		if err := config.SaveToFile(cfg, ws.ConfigPath); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
		return nil
	},
}

func getConfigValue(cfg *cobot.Config, key string) string {
	switch strings.ToLower(key) {
	case "model":
		return cfg.Model
	case "maxturns", "max_turns":
		return fmt.Sprintf("%d", cfg.MaxTurns)
	case "systemprompt", "system_prompt":
		return cfg.SystemPrompt
	case "temperature":
		return fmt.Sprintf("%.2f", cfg.Temperature)
	case "workspace":
		return cfg.Workspace
	case "verbose":
		return fmt.Sprintf("%t", cfg.Verbose)
	default:
		if strings.HasPrefix(key, "apikey.") {
			provider := strings.TrimPrefix(key, "apikey.")
			if v, ok := cfg.APIKeys[provider]; ok {
				return v
			}
		}
		return ""
	}
}

func setConfigValue(cfg *cobot.Config, key, value string) error {
	switch strings.ToLower(key) {
	case "model":
		cfg.Model = value
	case "maxturns", "max_turns":
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid max_turns value: %s", value)
		}
		cfg.MaxTurns = n
	case "systemprompt", "system_prompt":
		cfg.SystemPrompt = value
	case "temperature":
		var f float64
		if _, err := fmt.Sscanf(value, "%f", &f); err != nil {
			return fmt.Errorf("invalid temperature value: %s", value)
		}
		cfg.Temperature = f
	case "verbose":
		cfg.Verbose = strings.ToLower(value) == "true" || value == "1"
	default:
		if strings.HasPrefix(key, "apikey.") {
			provider := strings.TrimPrefix(key, "apikey.")
			if cfg.APIKeys == nil {
				cfg.APIKeys = make(map[string]string)
			}
			cfg.APIKeys[provider] = value
		} else {
			return fmt.Errorf("unknown config key: %s", key)
		}
	}
	return nil
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
