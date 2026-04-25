package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"myplatform/internal/runtime"
	"myplatform/internal/stack"
	"myplatform/internal/utils"
)

var stackRemoveVolumesFlag bool

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Manage multi-service stacks defined in YAML files",
}

var stackDeployCmd = &cobra.Command{
	Use:   "deploy <stack.yaml>",
	Short: "Deploy all services defined in a stack file",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackDeploy,
}

var stackRemoveCmd = &cobra.Command{
	Use:   "remove <stack.yaml>",
	Short: "Stop and remove all services defined in a stack file",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackRemove,
}

var stackStatusCmd = &cobra.Command{
	Use:   "status <stack.yaml>",
	Short: "Show runtime status of all services in a stack file",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackStatus,
}

var stackValidateCmd = &cobra.Command{
	Use:   "validate <stack.yaml>",
	Short: "Validate a stack file without deploying anything",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackValidate,
}

func init() {
	stackRemoveCmd.Flags().BoolVarP(
		&stackRemoveVolumesFlag, "volumes", "v", false,
		"Also remove named volumes declared in the stack file (data will be lost)",
	)
	stackCmd.AddCommand(stackDeployCmd, stackRemoveCmd, stackStatusCmd, stackValidateCmd)
	rootCmd.AddCommand(stackCmd)
}

func parseAndValidateStack(path string) (*stack.Stack, error) {
	s, err := stack.Parse(path)
	if err != nil {
		return nil, err
	}
	if err := stack.Validate(s); err != nil {
		return nil, err
	}
	return s, nil
}

func runStackDeploy(cmd *cobra.Command, args []string) error {
	s, err := parseAndValidateStack(args[0])
	if err != nil {
		return err
	}

	rt, err := runtime.NewDockerClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer rt.Close()

	utils.PrintInfo(fmt.Sprintf("Deploying stack %q (%d service(s))...", s.Name, len(s.Services)))
	if err := stack.Deploy(cmd.Context(), rt, s); err != nil {
		return err
	}
	utils.PrintSuccess(fmt.Sprintf("Stack %q deployed", s.Name))
	return nil
}

func runStackRemove(cmd *cobra.Command, args []string) error {
	s, err := parseAndValidateStack(args[0])
	if err != nil {
		return err
	}

	rt, err := runtime.NewDockerClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer rt.Close()

	utils.PrintInfo(fmt.Sprintf("Removing stack %q...", s.Name))
	if err := stack.Remove(cmd.Context(), rt, s, stackRemoveVolumesFlag); err != nil {
		return err
	}
	utils.PrintSuccess(fmt.Sprintf("Stack %q removed", s.Name))
	return nil
}

func runStackStatus(cmd *cobra.Command, args []string) error {
	s, err := parseAndValidateStack(args[0])
	if err != nil {
		return err
	}

	rt, err := runtime.NewDockerClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer rt.Close()

	fmt.Printf("Stack: %s\n\n", s.Name)
	return stack.Status(cmd.Context(), rt, s)
}

func runStackValidate(cmd *cobra.Command, args []string) error {
	s, err := parseAndValidateStack(args[0])
	if err != nil {
		return err
	}

	// Resolve deployment order — this also catches circular dependency errors.
	ordered, err := stack.ResolveOrder(s.Services)
	if err != nil {
		return err
	}

	utils.PrintSuccess(fmt.Sprintf("Stack %q is valid (%d service(s))", s.Name, len(s.Services)))
	for _, ns := range s.Services {
		fmt.Printf("  %-20s  image=%-30s  container=%s\n", ns.Key, ns.Def.Image, ns.Def.ContainerName)
	}

	fmt.Println()
	utils.PrintInfo("Deployment order:")
	for i, ns := range ordered {
		dep := ""
		if len(ns.Def.DependsOn) > 0 {
			dep = fmt.Sprintf("  (depends on: %s)", strings.Join(ns.Def.DependsOn, ", "))
		}
		fmt.Printf("  %d. %s%s\n", i+1, ns.Key, dep)
	}
	return nil
}
