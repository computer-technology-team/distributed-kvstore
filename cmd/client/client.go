package client

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewClientCmd creates a new client command
func NewClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Interact with the KVStore",
		Long:  "This command allows you to interact with the KVStore. You can set, get, delete, and check the existence of keys.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Get server URL from viper
			serverURL := viper.GetString("client.server_url")
			if serverURL == "" {
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

func createClient() (*kvstore.ClientWithResponses, error) {
	serverURL := viper.GetString("client.server_url")
	client, err := kvstore.NewClientWithResponses(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return client, nil
}
