package client

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/spf13/cobra"
)

// NewGetCmd creates a new get command
func NewGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Get the value of a key",
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
				return fmt.Errorf("failed to get key: %w", err)
			}

			if resp.StatusCode() == 404 {
				fmt.Printf("Key '%s' does not exist\n", key)
				return nil
			}

			if resp.StatusCode() != 200 {
				if resp.JSONDefault != nil {
					return fmt.Errorf("error getting key: %s", resp.JSONDefault.Error)
				}
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
			}

			fmt.Printf("Value for key '%s': '%s'\n", key, resp.JSON200.Value.MustGet())
			return nil
		},
	}
}
