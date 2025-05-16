package cmd

import (
	"fmt"
	"net"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/api/controller"
	apiKVStore "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/computer-technology-team/distributed-kvstore/internal/health"
	"github.com/computer-technology-team/distributed-kvstore/internal/node"
)

func NewServeNodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "servenode",
		Short: "Runs the database node of KVStore",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			cfg, err := config.LoadConfig(cmd.Flags())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			addr := fmt.Sprintf("%s:%d", cfg.Node.Host, cfg.Node.Port)

			client, err := controller.NewClientWithResponses(cfg.Node.ControllerURL)
			if err != nil {
				return fmt.Errorf("fail to create controller client: %w", err)
			}

			resp, err := client.PostNodesRegisterWithResponse(ctx, controller.NodeRegistration{Address: addr})
			if err != nil {
				return fmt.Errorf("failed to regsiter node: %w", err)
			}

			server := node.NewServer(resp.JSON201.Id)

			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}

			// Create a mux to handle both API and health check endpoints
			mux := http.NewServeMux()

			// Add the API handler
			mux.Handle("/", apiKVStore.Handler(apiKVStore.NewStrictHandler(server, nil)))

			// Add health check endpoint
			health.AddHealthCheckEndpoint(mux)

			fmt.Printf("Node server started at %s with health check at /health\n", addr)

			return http.Serve(listener, mux)
		},
	}
}
