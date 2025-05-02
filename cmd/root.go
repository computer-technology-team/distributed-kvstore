package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/cmd/flags"
	"github.com/computer-technology-team/distributed-kvstore/config"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dist-kv",
		Short: "Run and Manage Distributed KV Store",
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

	RegisterCommandRecursive(cmd)

	return cmd
}

func RegisterCommandRecursive(parent *cobra.Command) {
	versionCmd := NewVersionCmd()
	toolsCmd := NewToolsCmd()

	serveNodeCmd := NewServeNodeCmd()

	parent.AddCommand(versionCmd, toolsCmd, serveNodeCmd)
}

func Execute() {
	err := NewRootCmd().Execute()
	if err != nil {
		slog.Error("error in executing command", "error", err)
		os.Exit(-1)
	}
}
