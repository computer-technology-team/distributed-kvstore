package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/computer-technology-team/distributed-kvstore/cmd/client"
	"github.com/computer-technology-team/distributed-kvstore/config"
)

func NewRootCmd() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "distributed-kvstore",
		Short: "A distributed key-value store",
		Long: `A distributed key-value store with leader election and replication.
	This application provides a simple interface for storing and retrieving data across a cluster of nodes.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(nil)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: cfg.LogLevel.Level,
			})
			slog.SetDefault(slog.New(logHandler))
			slog.Info("Configuration loaded successfully")

			// Bind all flags to viper
			if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
				return fmt.Errorf("failed to bind flags: %w", err)
			}

			return nil
		},
	}

	versionCmd := NewVersionCmd()
	toolsCmd := NewToolsCmd()
	clientCmd := client.NewClientCmd()

	serveNodeCmd := NewServeNodeCmd()
	serveLoadBalancerCmd := NewServeLoadBalancerCmd()

	// controllerCmd := NewControllerCmd()
	rootCmd.AddCommand(versionCmd, toolsCmd, clientCmd, serveNodeCmd, serveLoadBalancerCmd)

	return rootCmd
}

func Execute() {
	err := NewRootCmd().Execute()
	if err != nil {
		slog.Error("error in executing command", "error", err)
		os.Exit(-1)
	}
}
