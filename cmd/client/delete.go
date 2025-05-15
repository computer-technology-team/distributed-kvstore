package client

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/spf13/cobra"
)

// NewDeleteCmd creates a new delete command
func NewDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [key]",
		Short: "Delete a key-value pair",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.LoadConfig(cmd)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			key := args[0]

			client, err := createClient(cfg.Client.ServerURL)
			if err != nil {
				return err
			}

			resp, err := client.DeleteKeyWithResponse(ctx, key)

			if err != nil {
				return fmt.Errorf("failed to delete key: %w", err)
			}

			if resp.StatusCode() == 404 {
				fmt.Printf("Key '%s' does not exist\n", key)
				return nil
			}

			if resp.StatusCode() != 200 {
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
			}

			fmt.Printf("Key '%s' deleted\n", key)
			return nil
		},
	}
}
