package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

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

		// Apply per-chat flags (model override and system prompt) if provided
		if m, _ := cmd.Flags().GetString("model"); m != "" {
			// Local per-chat model override
			ApplyChatFlags(cfg, m, "")
		}
		if p, _ := cmd.Flags().GetString("prompt"); p != "" {
			// Local per-chat system prompt override
			ApplyChatFlags(cfg, "", p)
		}

		a, cleanup, err := initAgent(cfg, true)
		if err != nil {
			return err
		}
		defer cleanup()

		agt, err := cobot.New(cfg, a)
		if err != nil {
			return err
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

// ApplyChatFlags allows per-chat overrides for a running chat session without
// mutating the global/root defaults. If modelFlag is non-empty, it overrides
// cfg.Model. If promptFlag is non-empty, it overrides cfg.SystemPrompt.
// This is intentionally exported to enable unit tests to verify the behavior.
func ApplyChatFlags(cfg *cobot.Config, modelFlag string, promptFlag string) {
	if cfg == nil {
		return
	}
	if modelFlag != "" {
		cfg.Model = modelFlag
	}
	if promptFlag != "" {
		cfg.SystemPrompt = promptFlag
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
	rootCmd.AddCommand(chatCmd)
	// Local chat session flags (non-persistent)
	chatCmd.Flags().String("model", "", "Model to use for this chat (overrides root default)")
	chatCmd.Flags().String("prompt", "", "System prompt template to apply for this chat")
}
