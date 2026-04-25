package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"myplatform/internal/runtime"
	"myplatform/internal/utils"
)

var statusNameFlag string

var statusCmd = &cobra.Command{
	Use:   "status <service|container>",
	Short: "Show the runtime status of a deployed service instance",
	Long: `Show the runtime status of a deployed service instance.

The argument can be:
  - A service name                  myplatform status mongodb
  - A full container name from list myplatform status myplatform-mongodb-2
  - A service name + --name flag    myplatform status mongodb --name staging`,
	Args: cobra.ExactArgs(1),
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVarP(
		&statusNameFlag, "name", "n", "",
		"Instance suffix used at deploy time (only needed with a service name argument)",
	)
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	target, err := parseInstanceArg(args[0], statusNameFlag)
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
		utils.PrintWarning(fmt.Sprintf("container %q has not been deployed", target.containerName))
		return nil
	}

	info, err := rt.InspectContainer(ctx, target.containerName)
	if err != nil {
		return err
	}

	fmt.Printf("  Service    : %s\n", target.cfg.Name)
	fmt.Printf("  Container  : %s\n", info.Name)
	fmt.Printf("  Image      : %s\n", info.Image)
	fmt.Printf("  State      : %s\n", info.Status)
	if info.Ports != "" {
		fmt.Printf("  Ports      : %s\n", info.Ports)
	}
	fmt.Println()

	if info.Running {
		utils.PrintSuccess("Service is running")
	} else {
		utils.PrintWarning("Service is stopped")
	}
	return nil
}
