package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

const configFileFlag = "config"

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dist-kv",
		Short: "Run and Manage Distributed KV Store",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
			slog.SetDefault(slog.New(logHandler))
		},
	}

	cmd.PersistentFlags().String(configFileFlag, "", "config file name")

	RegisterCommandRecursive(cmd)

	return cmd
}

func RegisterCommandRecursive(parent *cobra.Command) {
	parent.AddCommand()
}

func Execute() {
	err := NewRootCmd().Execute()
	if err != nil {
		slog.Error("error in executing command", "error", err)
		os.Exit(-1)
	}
}
