package tools

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"

	apiKVStore "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/cmd/flags"
	"github.com/computer-technology-team/distributed-kvstore/config"
)

func NewPingKVStoreCmd() *cobra.Command {
	return &cobra.Command{
		Use: "ping-kvstore",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString(flags.ConfigFileFlag)

			cfg, err := config.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			client, err := apiKVStore.NewClientWithResponses(
				fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port),
			)
			if err != nil {
				return fmt.Errorf("failed to create KV store client: %w", err)
			}

			resp, err := client.GetPingWithResponse(context.Background())
			if err != nil {
				return fmt.Errorf("failed to ping KV store: %w", err)
			}

			if resp.StatusCode() == http.StatusOK {
				slog.Info("Successfully pinged KV store", "status", resp.StatusCode())
			} else {
				slog.Error("Failed to ping KV store", "status", resp.StatusCode())
			}

			return nil
		},
	}
}
