package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/cmd/flags"
	"github.com/computer-technology-team/distributed-kvstore/config"
)

var rootCmd = &cobra.Command{
	Use:   "distributed-kvstore",
	Short: "A distributed key-value store",
	Long: `A distributed key-value store with leader election and replication.
	This application provides a simple interface for storing and retrieving data across a cluster of nodes.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configFile, _ := cmd.Flags().GetString(flags.ConfigFileFlag)
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: cfg.LogLevel,
		})
		slog.SetDefault(slog.New(logHandler))
		slog.Info("Configuration loaded successfully")
		return nil
	},
}

func NewRootCmd() *cobra.Command {
	RegisterCommandRecursive(rootCmd)
	return rootCmd
}

func RegisterCommandRecursive(parent *cobra.Command) {
	versionCmd := NewVersionCmd()
	toolsCmd := NewToolsCmd()
	kvStoreCmd := NewKVStoreCmd()

	serveNodeCmd := NewServeNodeCmd()

	parent.AddCommand(versionCmd, toolsCmd, kvStoreCmd, serveNodeCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		slog.Error("error in executing command", "error", err)
		os.Exit(-1)
	}
}
