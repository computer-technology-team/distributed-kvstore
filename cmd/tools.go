package cmd

import (
	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/cmd/tools"
)

func NewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "misc tools",
	}

	// Add tools subcommands
	pingKVStoreCmd := tools.NewPingKVStoreCmd()
	cmd.AddCommand(pingKVStoreCmd)

	return cmd
}
