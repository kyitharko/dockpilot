package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"dockpilot/internal/docker"
	"dockpilot/internal/engine"
)

var logsTailFlag int

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "Fetch logs from a deployed service container",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().IntVar(&logsTailFlag, "tail", 100,
		"Number of lines to show from the end of the logs")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	eng := engine.New(dc)
	name := strings.ToLower(args[0])

	lines, err := eng.Logs(cmd.Context(), name, logsTailFlag)
	if err != nil {
		return err
	}

	fmt.Println(strings.Join(lines, "\n"))
	return nil
}
