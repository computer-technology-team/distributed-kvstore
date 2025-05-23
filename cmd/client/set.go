package client

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/config"
)

// NewSetCmd creates a new set command
func NewSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a key-value pair",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, value := args[0], args[1]

			cfg, err := config.LoadConfig(cmd.Flags())
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			client, err := createClient(cfg.Client.ServerURL)
			if err != nil {
				return err
			}

			resp, err := client.SetValueWithResponse(ctx, key, kvstore.SetValueJSONRequestBody{
				Value: value,
			})

			if err != nil {
				return fmt.Errorf("failed to set key: %w", err)
			}

			if resp.StatusCode() != 200 {
				if resp.JSON400 != nil {
					return fmt.Errorf("error setting key: %s", resp.JSON400.Error)
				}
				if resp.JSONDefault != nil {
					return fmt.Errorf("error setting key: %s", resp.JSONDefault.Error)
				}
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
			}

			fmt.Printf("Key '%s' set to value '%s'\n", key, value)
			return nil
		},
	}
}
