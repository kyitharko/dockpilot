package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"dockpilot/internal/docker"
	"dockpilot/internal/engine"
	"dockpilot/internal/utils"
)

var statusCmd = &cobra.Command{
	Use:   "status <service>",
	Short: "Show runtime status of a deployed service",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	eng := engine.New(dc)
	name := strings.ToLower(args[0])

	status, err := eng.Status(cmd.Context(), name)
	if err != nil {
		return err
	}

	fmt.Printf("  Service    : %s\n", status.Name)
	fmt.Printf("  Container  : %s\n", status.Container)
	fmt.Printf("  Image      : %s\n", status.Image)
	fmt.Printf("  State      : %s\n", status.State)
	if status.Ports != "" {
		fmt.Printf("  Ports      : %s\n", status.Ports)
	}
	fmt.Println()

	if status.Running {
		utils.PrintSuccess("Service is running")
	} else if status.State == "not deployed" {
		utils.PrintWarning("Service has not been deployed")
	} else {
		utils.PrintWarning("Service is stopped")
	}
	return nil
}
