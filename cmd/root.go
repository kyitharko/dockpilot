package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"dockpilot/internal/docker"
	"dockpilot/internal/utils"
)

var rootCmd = &cobra.Command{
	Use:   "dockpilot",
	Short: "Low-level Docker service executor",
	Long: `dockpilot deploys and manages individual Docker containers using the Docker SDK.

Built-in services: mongodb, postgres, redis, nginx
Custom images:     dockpilot deploy myapp --image nginx:alpine

Start the REST API server:
  dockpilot server --port 8088`,

	SilenceErrors: true,
	SilenceUsage:  true,

	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		return docker.CheckDaemon(cmd.Context())
	},
}

// Execute is the single entry point called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		utils.PrintError(err.Error())
		os.Exit(1)
	}
}
