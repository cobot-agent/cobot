package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/acp"
	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/llm/openai"
	"github.com/cobot-agent/cobot/internal/memory"
	"github.com/cobot-agent/cobot/internal/xdg"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "ACP server commands",
}

var acpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start ACP server (stdio mode)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		a := agent.New(cfg)

		apiKey := cfg.APIKeys["openai"]
		if apiKey != "" {
			provider := openai.NewProvider(apiKey, "")
			a.SetProvider(provider)
		} else {
			fmt.Fprintf(os.Stderr, "warning: no OpenAI API key configured; agent calls will fail\n")
		}

		dataDir := filepath.Join(xdg.DataHome(), "cobot")
		memDir := filepath.Join(dataDir, "memory")
		memStore, err := memory.OpenStore(memDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to open memory store: %v\n", err)
		} else {
			a.SetMemoryStore(memStore)
		}

		srv := acp.NewACPServer(a)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		defer a.Close()

		return srv.Run(ctx)
	},
}

func init() {
	acpCmd.AddCommand(acpServeCmd)
	rootCmd.AddCommand(acpCmd)
}
