package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/memory"
	"github.com/cobot-agent/cobot/internal/xdg"
	cobot "github.com/cobot-agent/cobot/pkg"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Search and inspect memory palace",
}

var memorySearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search memory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
		store, err := memory.OpenStore(dataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		wingID, _ := cmd.Flags().GetString("wing")
		results, err := store.Search(context.Background(), &cobot.SearchQuery{
			Text:   args[0],
			WingID: wingID,
			Limit:  10,
		})
		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No results found.")
			return nil
		}

		for _, r := range results {
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %.2f %s\n", r.DrawerID, r.Score, truncate(r.Content, 120))
		}
		return nil
	},
}

var memoryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show memory palace overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := filepath.Join(xdg.DataHome(), "cobot", "memory")
		store, err := memory.OpenStore(dataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		wings, err := store.GetWings(context.Background())
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Memory Palace: %d wings\n", len(wings))
		for _, w := range wings {
			rooms, _ := store.GetRooms(context.Background(), w.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "  Wing: %s (%s) — %d rooms\n", w.Name, w.ID, len(rooms))
			for _, r := range rooms {
				fmt.Fprintf(cmd.OutOrStdout(), "    Room: %s [%s]\n", r.Name, r.HallType)
			}
		}
		return nil
	},
}

func init() {
	memorySearchCmd.Flags().StringP("wing", "w", "", "Filter by wing ID")
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memoryStatusCmd)
	rootCmd.AddCommand(memoryCmd)
}
