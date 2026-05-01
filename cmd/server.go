package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"dockpilot/internal/api"
	"dockpilot/internal/docker"
	"dockpilot/internal/engine"
	"dockpilot/internal/utils"
)

var (
	serverHostFlag string
	serverPortFlag int
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the dockpilot REST API server",
	Long: `Start an HTTP server exposing the dockpilot REST API.

Endpoints:
  GET    /health                          — Docker daemon health check
  GET    /v1/services                     — List built-in service definitions
  POST   /v1/services/{service}/deploy    — Deploy a service container
  DELETE /v1/services/{service}           — Stop and remove a container
  GET    /v1/services/{service}/status    — Runtime status of a service
  GET    /v1/services/{service}/logs      — Fetch container logs

Binds to 127.0.0.1 by default (local-only). Use --host 0.0.0.0 to expose publicly.`,
	RunE: runServer,
}

func init() {
	serverCmd.Flags().StringVar(&serverHostFlag, "host", "127.0.0.1",
		"Interface to bind to (use 0.0.0.0 to listen on all interfaces)")
	serverCmd.Flags().IntVar(&serverPortFlag, "port", 8088,
		"Port to listen on")
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, _ []string) error {
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	eng := engine.New(dc)
	addr := fmt.Sprintf("%s:%d", serverHostFlag, serverPortFlag)

	// Capture SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	utils.PrintInfo(fmt.Sprintf("dockpilot API server listening on http://%s", addr))
	utils.PrintInfo("Press Ctrl+C to stop")

	srv := api.New(eng, addr)
	return srv.Serve(ctx)
}
