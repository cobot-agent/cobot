package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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

		m, err := workspace.NewManager()
		if err != nil {
			return fmt.Errorf("create workspace manager: %w", err)
		}
		ws, err := m.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		cfg := cobot.DefaultConfig()
		_ = config.LoadFromFile(cfg, ws.ConfigPath())

		if err := setConfigValue(cfg, key, value); err != nil {
			return err
		}

		if err := config.SaveToFile(cfg, ws.ConfigPath()); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long:  "Create a new configuration file with interactive prompts or default values",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		m, err := workspace.NewManager()
		if err != nil {
			return fmt.Errorf("create workspace manager: %w", err)
		}
		ws, err := m.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		if _, err := os.Stat(ws.ConfigPath()); err == nil && !force {
			return fmt.Errorf("config already exists at %s (use --force to overwrite)", ws.ConfigPath())
		}

		cfg := cobot.DefaultConfig()

		_ = config.LoadFromFile(cfg, ws.ConfigPath())

		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			if err := interactiveConfig(cfg); err != nil {
				return err
			}
		}

		if err := config.SaveToFile(cfg, ws.ConfigPath()); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Configuration initialized at %s\n", ws.ConfigPath())
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file in default editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := workspace.NewManager()
		if err != nil {
			return fmt.Errorf("create workspace manager: %w", err)
		}
		ws, err := m.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		if _, err := os.Stat(ws.ConfigPath()); os.IsNotExist(err) {
			cfg := cobot.DefaultConfig()
			if err := config.SaveToFile(cfg, ws.ConfigPath()); err != nil {
				return fmt.Errorf("creating config: %w", err)
			}
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		execCmd := execCommand(editor, ws.ConfigPath())
		execCmd.Stdin = os.Stdin
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr

		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}

		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := workspace.NewManager()
		if err != nil {
			return fmt.Errorf("create workspace manager: %w", err)
		}
		ws, err := m.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		cfg := cobot.DefaultConfig()
		if err := config.LoadFromFile(cfg, ws.ConfigPath()); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}

		issues := validateConfig(cfg)
		if len(issues) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration issues found:")
			for _, issue := range issues {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", issue)
			}
			return fmt.Errorf("config validation failed")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Configuration is valid")
		return nil
	},
}

var configSetProviderCmd = &cobra.Command{
	Use:   "set-provider <provider>",
	Short: "Configure LLM provider settings",
	Long:  "Configure provider settings like base URL for OpenAI-compatible APIs (Ollama, vLLM, etc.)",
	Example: `  cobot config set-provider openai --base-url http://localhost:11434/v1
  cobot config set-provider openai --base-url https://api.openai.com/v1 --header "Authorization: Bearer token"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		providerName := args[0]
		baseURL, _ := cmd.Flags().GetString("base-url")
		headerFlags, _ := cmd.Flags().GetStringArray("header")

		m, err := workspace.NewManager()
		if err != nil {
			return fmt.Errorf("create workspace manager: %w", err)
		}
		ws, err := m.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		cfg := cobot.DefaultConfig()
		_ = config.LoadFromFile(cfg, ws.ConfigPath())

		if cfg.Providers == nil {
			cfg.Providers = make(map[string]cobot.ProviderConfig)
		}

		pc := cfg.Providers[providerName]

		if baseURL != "" {
			pc.BaseURL = baseURL
		}

		if len(headerFlags) > 0 {
			if pc.Headers == nil {
				pc.Headers = make(map[string]string)
			}
			for _, h := range headerFlags {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					pc.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		}

		cfg.Providers[providerName] = pc

		if err := config.SaveToFile(cfg, ws.ConfigPath()); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Configured provider: %s\n", providerName)
		if pc.BaseURL != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Base URL: %s\n", pc.BaseURL)
		}
		if len(pc.Headers) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  Headers:")
			for k := range pc.Headers {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", k)
			}
		}
		return nil
	},
}

var configSetApiKeyCmd = &cobra.Command{
	Use:   "set-apikey <provider> <key>",
	Short: "Set API key for a provider",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := args[0]
		apiKey := args[1]

		m, err := workspace.NewManager()
		if err != nil {
			return fmt.Errorf("create workspace manager: %w", err)
		}
		ws, err := m.ResolveByNameOrDiscover("", ".")
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}

		cfg := cobot.DefaultConfig()
		_ = config.LoadFromFile(cfg, ws.ConfigPath())

		if cfg.APIKeys == nil {
			cfg.APIKeys = make(map[string]string)
		}
		cfg.APIKeys[provider] = apiKey

		if err := config.SaveToFile(cfg, ws.ConfigPath()); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Set API key for %s\n", provider)
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

func interactiveConfig(cfg *cobot.Config) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Configuration Wizard")
	fmt.Println("===================")
	fmt.Println()

	fmt.Printf("Model [%s]: ", cfg.Model)
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		cfg.Model = strings.TrimSpace(input)
	}

	fmt.Printf("Max turns [%d]: ", cfg.MaxTurns)
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(input)); err == nil {
			cfg.MaxTurns = n
		}
	}

	fmt.Printf("Temperature [%.1f]: ", cfg.Temperature)
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(input), 64); err == nil {
			cfg.Temperature = f
		}
	}

	fmt.Println()
	fmt.Println("API Keys (press Enter to skip):")

	fmt.Print("OpenAI API key: ")
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		if cfg.APIKeys == nil {
			cfg.APIKeys = make(map[string]string)
		}
		cfg.APIKeys["openai"] = strings.TrimSpace(input)
	}

	fmt.Print("Anthropic API key: ")
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		if cfg.APIKeys == nil {
			cfg.APIKeys = make(map[string]string)
		}
		cfg.APIKeys["anthropic"] = strings.TrimSpace(input)
	}

	return nil
}

func validateConfig(cfg *cobot.Config) []string {
	var issues []string

	if cfg.Model == "" {
		issues = append(issues, "model is not set")
	}

	if cfg.MaxTurns <= 0 {
		issues = append(issues, "max_turns must be positive")
	}

	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		issues = append(issues, "temperature should be between 0 and 2")
	}

	return issues
}

var execCommand = exec.Command

func init() {
	configInitCmd.Flags().Bool("interactive", false, "Interactive mode with prompts")
	configInitCmd.Flags().Bool("force", false, "Overwrite existing config")
	configSetProviderCmd.Flags().String("base-url", "", "Base URL for the provider API (e.g., http://localhost:11434/v1 for Ollama)")
	configSetProviderCmd.Flags().StringArray("header", nil, "Custom headers (format: 'Key: Value')")

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configSetProviderCmd)
	configCmd.AddCommand(configSetApiKeyCmd)
	rootCmd.AddCommand(configCmd)
}
