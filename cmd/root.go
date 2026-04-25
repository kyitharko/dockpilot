package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"myplatform/internal/runtime"
	"myplatform/internal/utils"
)

// rootCmd is the base command. All subcommands attach to it via their init().
var rootCmd = &cobra.Command{
	Use:   "myplatform",
	Short: "CLI platform tool for deploying common services with Docker",
	Long: `myplatform deploys and manages Docker containers with sensible production defaults.
Supports built-in services (mongodb, postgres, redis, nginx) and any image from
Docker Hub or a private registry via the --image flag.`,

	SilenceErrors: true,
	SilenceUsage:  true,

	// PersistentPreRunE pings the Docker daemon before any subcommand runs.
	// If the daemon is unreachable the user gets a clear error immediately.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return runtime.CheckDaemon(cmd.Context())
	},
}

// Execute is the single entry point called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		utils.PrintError(err.Error())
		os.Exit(1)
	}
}
