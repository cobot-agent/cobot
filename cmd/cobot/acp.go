package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/acp"
	"github.com/cobot-agent/cobot/internal/agent"
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
		srv := acp.NewACPServer(a)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return srv.Run(ctx)
	},
}

func init() {
	acpCmd.AddCommand(acpServeCmd)
	rootCmd.AddCommand(acpCmd)
}
