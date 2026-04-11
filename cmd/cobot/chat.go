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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
	rootCmd.AddCommand(chatCmd)
}
