package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/computer-technology-team/distributed-kvstore/api/controller"
	apiKVStore "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	apiLoadBalancer "github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"

	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/computer-technology-team/distributed-kvstore/internal/health"
	"github.com/computer-technology-team/distributed-kvstore/internal/loadbalancer"
)

func NewServeLoadBalancerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "servebalancer",
		Short: "Runs the loadbalancer of KVStore",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			cfg, err := config.LoadConfig(cmd.Flags())
			if err != nil {
				slog.Error("Failed to load config", "error", err)
				return fmt.Errorf("failed to load config: %w", err)
			}

			client, err := controller.NewClientWithResponses(cfg.LoadBalancer.ControllerURL)
			if err != nil {
				return fmt.Errorf("failed to create controller client: %w", err)
			}

			server, err := loadbalancer.NewServer(ctx, client)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			publicAddr := fmt.Sprintf("%s:%d",
				cfg.LoadBalancer.PublicServer.Host, cfg.LoadBalancer.PublicServer.Port)

			privateAddr := fmt.Sprintf("%s:%d",
				cfg.LoadBalancer.PrivateServer.Host, cfg.LoadBalancer.PrivateServer.Port)

			publicListener, err := net.Listen("tcp", publicAddr)
			if err != nil {
				slog.Error("Failed to create public listener", "address", publicAddr, "error", err)
				return fmt.Errorf("failed to create public listener: %w", err)
			}

			privateListener, err := net.Listen("tcp", privateAddr)
			if err != nil {
				slog.Error("Failed to create private listener", "address", privateAddr, "error", err)
				return fmt.Errorf("failed to create private listener: %w", err)
			}

			// Create mux for public server to handle both API and health check endpoints
			publicMux := http.NewServeMux()
			publicMux.Handle("/", apiKVStore.Handler(apiKVStore.NewStrictHandler(server, nil)))
			health.AddHealthCheckEndpoint(publicMux)

			// Create mux for private server to handle both API and health check endpoints
			privateMux := http.NewServeMux()
			privateMux.Handle("/", apiLoadBalancer.Handler(apiLoadBalancer.NewStrictHandler(server, nil)))
			health.AddHealthCheckEndpoint(privateMux)

			slog.Info("Health check endpoints added to public and private servers at /health")

			publicServer := &http.Server{
				Handler: publicMux,
			}

			privateServer := &http.Server{
				Handler: privateMux,
			}

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				slog.Info("Public server listening", "address", publicAddr)
				if err := publicServer.Serve(publicListener); err != nil && err != http.ErrServerClosed {
					slog.Error("Public server error", "error", err)
				}
			}()

			go func() {
				defer wg.Done()
				slog.Info("Private server listening", "address", privateAddr)
				if err := privateServer.Serve(privateListener); err != nil && err != http.ErrServerClosed {
					slog.Error("Private server error", "error", err)
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

			<-stop
			slog.Info("Shutting down servers...")

			// Graceful shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := publicServer.Shutdown(ctx); err != nil {
				slog.Error("Public server shutdown error", "error", err)
			}

			if err := privateServer.Shutdown(ctx); err != nil {
				slog.Error("Private server shutdown error", "error", err)
			}

			wg.Wait()
			slog.Info("Servers gracefully stopped")

			return nil
		},
	}
}
