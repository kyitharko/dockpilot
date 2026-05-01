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

var (
	deployImageFlag string
	deployPortFlags []string
	deployEnvFlags  []string
)

var deployCmd = &cobra.Command{
	Use:   "deploy <service>",
	Short: "Deploy a service container",
	Long: `Deploy a managed service container using Docker SDK.

Built-in services (no --image needed): ` + strings.Join(services.Names(), ", ") + `

Custom image examples:
  dockpilot deploy myapp --image nginx:alpine
  dockpilot deploy myapp --image nginx:alpine --port 8080:80 --env DEBUG=1`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: services.Names(),
	RunE:      runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployImageFlag, "image", "",
		"Docker image to deploy (required for custom services)")
	deployCmd.Flags().StringArrayVar(&deployPortFlags, "port", nil,
		"Port mapping host:container, repeatable (e.g. --port 8080:80)")
	deployCmd.Flags().StringArrayVar(&deployEnvFlags, "env", nil,
		"Environment variable KEY=VALUE, repeatable (e.g. --env DEBUG=1)")
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	eng := engine.New(dc)
	name := strings.ToLower(args[0])

	req := engine.DeployRequest{
		Name:  name,
		Image: deployImageFlag,
		Ports: deployPortFlags,
		Env:   deployEnvFlags,
	}

	utils.PrintInfo(fmt.Sprintf("Deploying service %q...", name))
	result, err := eng.Deploy(cmd.Context(), req)
	if err != nil {
		return err
	}

	utils.PrintSuccess(fmt.Sprintf("Deployed %q → container %q (image: %s)", name, result.Container, result.Image))
	if len(result.Ports) > 0 {
		utils.PrintInfo(fmt.Sprintf("Ports: %s", strings.Join(result.Ports, ", ")))
	}
	return nil
}
