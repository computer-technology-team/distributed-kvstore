package cmd

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"

	lbapi "github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
	"github.com/computer-technology-team/distributed-kvstore/cmd/flags"
	"github.com/computer-technology-team/distributed-kvstore/config"
	lbhandler "github.com/computer-technology-team/distributed-kvstore/internal/loadbalancer"
)

func NewServeLBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "servelb",
		Short: "Runs the load balancer service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString(flags.ConfigFileFlag)
			cfg, err := config.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(cfg.Replicas) == 0 {
				return fmt.Errorf("no replicas configured; please set 'replicas:' in config.yaml")
			}
			slog.Info("Loaded replicas", "replicas", cfg.Replicas)

			handler := lbhandler.New(cfg.Replicas)
			router := lbapi.Handler(handler)

			addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
			slog.Info("Starting load balancer", "address", addr)
			return http.ListenAndServe(addr, router)
		},
	}

	cmd.Flags().String(flags.ConfigFileFlag, "", "config file path")
	return cmd
}
