package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"dockpilot/internal/docker"
	"dockpilot/internal/engine"
	"dockpilot/internal/utils"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all dockpilot-managed service containers",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	eng := engine.New(dc)
	services, err := eng.List(cmd.Context())
	if err != nil {
		return err
	}

	if len(services) == 0 {
		utils.PrintInfo("No managed services found. Deploy one with: dockpilot deploy <service>")
		return nil
	}

	w := utils.NewTabWriter(os.Stdout)
	fmt.Fprintln(w, "NAME\tCONTAINER\tIMAGE\tSTATE\tPORTS")
	fmt.Fprintln(w, "----\t---------\t-----\t-----\t-----")
	for _, svc := range services {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			svc.Name, svc.Container, svc.Image, svc.State, svc.Ports)
	}
	return w.Flush()
}
