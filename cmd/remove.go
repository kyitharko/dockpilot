package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"myplatform/internal/runtime"
	"myplatform/internal/utils"
)

var (
	removeVolumesFlag bool
	removeNameFlag    string
)

var removeCmd = &cobra.Command{
	Use:   "remove <service|container>",
	Short: "Stop and remove a deployed service container",
	Long: `Stop and remove a deployed service container.

The argument can be:
  - A service name                  myplatform remove mongodb
  - A full container name from list myplatform remove myplatform-mongodb-2
  - A service name + --name flag    myplatform remove mongodb --name staging

Use --volumes to also delete the associated named volume (data will be lost).`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().BoolVarP(
		&removeVolumesFlag, "volumes", "v", false,
		"Also remove the named volume(s) associated with this instance",
	)
	removeCmd.Flags().StringVarP(
		&removeNameFlag, "name", "n", "",
		"Instance suffix used at deploy time (only needed with a service name argument)",
	)
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	target, err := parseInstanceArg(args[0], removeNameFlag)
	if err != nil {
		return err
	}

	rt, err := runtime.NewDockerClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer rt.Close()
	ctx := cmd.Context()

	exists, err := rt.ContainerExists(ctx, target.containerName)
	if err != nil {
		return err
	}
	if !exists {
		utils.PrintWarning(fmt.Sprintf("container %q does not exist -- nothing to remove", target.containerName))
		return nil
	}

	utils.PrintInfo(fmt.Sprintf("Stopping container %q...", target.containerName))
	if err := rt.StopContainer(ctx, target.containerName); err != nil {
		return err
	}

	utils.PrintInfo(fmt.Sprintf("Removing container %q...", target.containerName))
	if err := rt.RemoveContainer(ctx, target.containerName); err != nil {
		return err
	}
	utils.PrintSuccess(fmt.Sprintf("Container %q removed", target.containerName))

	if removeVolumesFlag {
		for _, vol := range target.volumes {
			volName := strings.SplitN(vol, ":", 2)[0]
			utils.PrintInfo(fmt.Sprintf("Removing volume %q...", volName))
			if err := rt.RemoveVolume(ctx, volName); err != nil {
				utils.PrintWarning(fmt.Sprintf("Could not remove volume %q: %v", volName, err))
			} else {
				utils.PrintSuccess(fmt.Sprintf("Volume %q removed", volName))
			}
		}
	}

	return nil
}
