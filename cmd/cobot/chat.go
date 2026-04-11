package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/llm/openai"
	"github.com/cobot-agent/cobot/internal/memory"
	"github.com/cobot-agent/cobot/internal/tools/builtin"
	"github.com/cobot-agent/cobot/internal/xdg"
	cobot "github.com/cobot-agent/cobot/pkg"
)

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a message to the agent",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		core := agent.New(cfg)
		agt, err := cobot.New(cfg, core)
		if err != nil {
			return err
		}
		defer agt.Close()

		agt.RegisterTool(builtin.NewReadFileTool())
		agt.RegisterTool(builtin.NewWriteFileTool())
		agt.RegisterTool(builtin.NewShellExecTool())

		apiKey := cfg.APIKeys["openai"]
		if apiKey == "" {
			return fmt.Errorf("openai API key not configured (set api_keys.openai in config or OPENAI_API_KEY env)")
		}

		provider := openai.NewProvider(apiKey, "")
		agt.SetProvider(provider)

		memDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
		if ms, err := memory.OpenStore(memDir); err == nil {
			core.SetMemoryStore(ms)
		}

		ch, err := agt.Stream(context.Background(), args[0])
		if err != nil {
			return err
		}

		for event := range ch {
			switch event.Type {
			case cobot.EventText:
				fmt.Print(event.Content)
			case cobot.EventToolCall:
				fmt.Fprintf(os.Stderr, "[Tool: %s]\n", event.ToolCall.Name)
			case cobot.EventToolResult:
				fmt.Fprintf(os.Stderr, "[Result: %s]\n", truncate(event.Content, 100))
			case cobot.EventDone:
				fmt.Println()
			case cobot.EventError:
				fmt.Fprintf(os.Stderr, "Error: %v\n", event.Error)
			}
		}
		return nil
	},
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
	rootCmd.AddCommand(chatCmd)
}
