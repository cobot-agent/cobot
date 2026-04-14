package main

import (
	"context"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/memory/daemon"
	"github.com/cobot-agent/cobot/internal/xdg"
)

var memorydCmd = &cobra.Command{
	Use:    "memoryd",
	Short:  "Memory daemon (internal, auto-started)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, _ := cmd.Flags().GetString("data")
		if dataDir == "" {
			dataDir = xdg.DataDir()
		}

		ctx, stop := signal.NotifyContext(context.Background(), InterruptSignals()...)
		defer stop()

		return daemon.ServeMemoryDaemon(ctx, dataDir)
	},
}

func init() {
	memorydCmd.Flags().String("data", "", "data directory")
	rootCmd.AddCommand(memorydCmd)
}
