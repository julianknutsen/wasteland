package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newVersionCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintf(stdout, "wl %s (commit: %s, built: %s)\n", version, commit, date)
			return nil
		},
	}
}
