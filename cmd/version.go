package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/config"
)

// NewVersionCmd creates a new version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Display the current version of the distributed key-value store.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Distributed KV Store version: %s\n", config.Version)
		},
	}
}
