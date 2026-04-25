package utils

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// ANSI escape codes for terminal colour.
const (
	reset  = "\033[0m"
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

// PrintSuccess prints a green success message to stdout.
func PrintSuccess(msg string) {
	fmt.Printf("%s✔%s  %s\n", green, reset, msg)
}

// PrintError prints a red error message to stderr.
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "%s✖%s  %s\n", red, reset, msg)
}

// PrintInfo prints a cyan informational message to stdout.
func PrintInfo(msg string) {
	fmt.Printf("%s→%s  %s\n", cyan, reset, msg)
}

// PrintWarning prints a yellow warning message to stdout.
func PrintWarning(msg string) {
	fmt.Printf("%s!%s  %s\n", yellow, reset, msg)
}

// NewTabWriter returns a tabwriter configured for clean column alignment.
// The caller must call Flush() when done.
func NewTabWriter(out io.Writer) *tabwriter.Writer {
	// minwidth=0, tabwidth=0, padding=3, padchar=' ', flags=0
	return tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
}
