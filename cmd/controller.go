package cmd

import (
	"context"
	"errors"
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

	"github.com/computer-technology-team/distributed-kvstore/config"
	"github.com/computer-technology-team/distributed-kvstore/internal/controller"
	"github.com/computer-technology-team/distributed-kvstore/internal/health"

	controllerAPI "github.com/computer-technology-team/distributed-kvstore/api/controller"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
)

func NewControllerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "controller",
		Short: "Runs the controller for the distributed KVStore",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadConfig(cmd.Flags())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			balancerClient, err := loadbalancer.NewClientWithResponses(cfg.Controller.LoadBalancerURL)
			if err != nil {
				return fmt.Errorf("failed to create balancer client: %w", err)
			}

			ctrl := controller.NewController(balancerClient)

			controllerAddr := fmt.Sprintf("%s:%d", cfg.Controller.Host, cfg.Controller.Port)
			controllerListener, err := net.Listen("tcp", controllerAddr)
			if err != nil {
				return fmt.Errorf("failed to start controller server: %w", err)
			}

			var controllerHttpServer, adminHttpServer http.Server

			var wg sync.WaitGroup

			if cfg.Controller.AdminUI.Enabled {
				wg.Add(1)
				go func() {
					defer wg.Done()
					adminUIAddr := fmt.Sprintf("%s:%d", cfg.Controller.AdminUI.Host, cfg.Controller.AdminUI.Port)
					slog.Info("Starting AdminUI server", "address", adminUIAddr)
					adminUIListener, err := net.Listen("tcp", adminUIAddr)
					if err != nil {
						slog.Error("failed to start adminui listener", "error", err)
						return
					}

					adminServer, err := controller.NewAdminServer(ctrl)
					if err != nil {
						slog.Error("could not create admin server", "error", err)
						return
					}

					err = http.Serve(adminUIListener, adminServer.Router())

					if err != nil && !errors.Is(err, http.ErrServerClosed) {
						slog.Error("could not serve admin-ui listener", "error", err)
					}
				}()
			}

			wg.Add(1)
			go func() {
				defer wg.Done()

				// Create mux for controller server to handle both API and health check endpoints
				controllerMux := http.NewServeMux()
				controllerMux.Handle("/", controllerAPI.Handler(
					controllerAPI.NewStrictHandler(controller.NewServer(ctrl), nil)))

				// Add health check endpoint
				health.AddHealthCheckEndpoint(controllerMux)

				slog.Info("Health check endpoint added to controller server at /health")

				controllerHttpServer.Handler = controllerMux
				err := controllerHttpServer.Serve(controllerListener)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("could not serve controller listener", "error", err)
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

			<-stop
			slog.Info("Shutting down servers...")

			// Graceful shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := controllerHttpServer.Shutdown(ctx); err != nil {
				slog.Error("Public server shutdown error", "error", err)
			}

			if cfg.Controller.AdminUI.Enabled {

				if err := adminHttpServer.Shutdown(ctx); err != nil {
					slog.Error("Private server shutdown error", "error", err)
				}
			}

			wg.Wait()
			slog.Info("Servers gracefully stopped")

			return nil
		},
	}
}
