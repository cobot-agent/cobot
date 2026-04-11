package main

import (
	"encoding/json"
	"fmt"

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

func init() {
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
