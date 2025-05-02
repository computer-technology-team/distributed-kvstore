package cmd

import (
	"fmt"
	"net"
	"net/http"

	"github.com/spf13/cobra"

	apiKVStore "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/cmd/flags"
	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/computer-technology-team/distributed-kvstore/internal/kvstore"
)

func NewServeNodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "servenode",
		Short: "Runs the database node of KVStore",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, _ := cmd.Flags().GetString(flags.ConfigFileFlag)

			cfg, err := config.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			server := kvstore.NewServer()

			addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}

			return http.Serve(listener, apiKVStore.Handler(apiKVStore.NewStrictHandler(server, nil)))
		},
	}
}
