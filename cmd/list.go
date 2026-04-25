package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"myplatform/internal/runtime"
	"myplatform/internal/utils"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed service containers",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	rt, err := runtime.NewDockerClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer rt.Close()

	containers, err := rt.ListManagedContainers(cmd.Context())
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		utils.PrintInfo("No managed services found. Deploy one with 'myplatform deploy <service>'.")
		return nil
	}

	w := utils.NewTabWriter(os.Stdout)
	fmt.Fprintln(w, "CONTAINER\tIMAGE\tSTATUS\tPORTS")
	fmt.Fprintln(w, "---------\t-----\t------\t-----")
	for _, c := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Name, c.Image, c.Status, c.Ports)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println()
	utils.PrintInfo("Use the CONTAINER name directly to manage any instance:")
	utils.PrintInfo("  myplatform status <container>   myplatform remove <container>")
	return nil
}
