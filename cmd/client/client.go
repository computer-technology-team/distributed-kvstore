package client

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/spf13/cobra"
)

// NewClientCmd creates a new client command
func NewClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Interact with the KVStore",
		Long:  "This command allows you to interact with the KVStore. You can set, get, delete, and check the existence of keys.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cmd.Flags())
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if cfg.Client.ServerURL == "" {
				return fmt.Errorf("server URL is required via --client.server-url flag or in configuration")
			}
			return nil
		},
	}

	cmd.AddCommand(
		NewSetCmd(),
		NewGetCmd(),
		NewDeleteCmd(),
		NewExistsCmd(),
	)

	return cmd
}

func createClient(serverURL string) (*kvstore.ClientWithResponses, error) {
	client, err := kvstore.NewClientWithResponses(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return client, nil
}
