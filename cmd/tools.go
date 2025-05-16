package cmd

import (
	"github.com/spf13/cobra"
)

func NewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "misc tools",
	}

	// // Add tools subcommands
	// pingKVStoreCmd := tools.NewPingKVStoreCmd()
	// cmd.AddCommand(pingKVStoreCmd)

	return cmd
}
