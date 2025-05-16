package client

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/spf13/cobra"
)

// NewExistsCmd creates a new exists command
func NewExistsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exists [key]",
		Short: "Check if a key exists",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key := args[0]

			cfg, err := config.LoadConfig(cmd.Flags())
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			client, err := createClient(cfg.Client.ServerURL)
			if err != nil {
				return err
			}

			resp, err := client.GetValueWithResponse(ctx, key)

			if err != nil {
				return fmt.Errorf("failed to check key: %w", err)
			}

			if resp.StatusCode() == 404 {
				fmt.Printf("Key '%s' does not exist\n", key)
				return nil
			}

			if resp.StatusCode() != 200 {
				if resp.JSONDefault != nil {
					return fmt.Errorf("error checking key: %s", resp.JSONDefault.Error)
				}
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
			}

			fmt.Printf("Key '%s' exists\n", key)
			return nil
		},
	}
}
