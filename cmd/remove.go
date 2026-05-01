package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"dockpilot/internal/docker"
	"dockpilot/internal/engine"
	"dockpilot/internal/services"
	"dockpilot/internal/utils"
)

var removeVolumesFlag bool

var removeCmd = &cobra.Command{
	Use:   "remove <service>",
	Short: "Stop and remove a deployed service container",
	Long: `Stop and remove a deployed service container.

Use --volumes to also delete the named volumes associated with the service.
Volume names for built-in services are resolved automatically from the registry.`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().BoolVarP(&removeVolumesFlag, "volumes", "v", false,
		"Also remove the named volumes associated with this service (data will be lost)")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	eng := engine.New(dc)
	name := strings.ToLower(args[0])

	var volumes []string
	if removeVolumesFlag {
		if svc, ok := services.Get(name); ok {
			for _, vol := range svc.Volumes {
				volumes = append(volumes, strings.SplitN(vol, ":", 2)[0])
			}
		}
	}

	utils.PrintInfo(fmt.Sprintf("Removing service %q...", name))
	if err := eng.Remove(cmd.Context(), name, volumes); err != nil {
		return err
	}
	utils.PrintSuccess(fmt.Sprintf("Service %q removed", name))
	return nil
}
