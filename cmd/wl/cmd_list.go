package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
)

func newListCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List joined wastelands",
		Long: `List all wastelands this rig has joined.

Displays upstream path, rig handle, fork, local directory, and join date
for each wasteland.

Examples:
  wl list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(stdout, stderr)
		},
	}
}

func runList(stdout, stderr io.Writer) error {
	store := federation.NewConfigStore()

	upstreams, err := store.List()
	if err != nil {
		return fmt.Errorf("listing wastelands: %w", err)
	}

	if len(upstreams) == 0 {
		fmt.Fprintf(stdout, "No wastelands joined.\n")
		fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Join one with: wl join <org/db>"))
		return nil
	}

	fmt.Fprintf(stdout, "Joined wastelands (%d):\n\n", len(upstreams))

	for _, upstream := range upstreams {
		cfg, err := store.Load(upstream)
		if err != nil {
			fmt.Fprintf(stderr, "  %s: error loading config: %v\n", upstream, err)
			continue
		}

		fmt.Fprintf(stdout, "  %s\n", style.Bold.Render(cfg.Upstream))
		fmt.Fprintf(stdout, "    Handle:  %s\n", cfg.RigHandle)
		fmt.Fprintf(stdout, "    Fork:    %s/%s\n", cfg.ForkOrg, cfg.ForkDB)
		fmt.Fprintf(stdout, "    Local:   %s\n", cfg.LocalDir)
		if !cfg.JoinedAt.IsZero() {
			fmt.Fprintf(stdout, "    Joined:  %s\n", cfg.JoinedAt.Format("2006-01-02"))
		}
		fmt.Fprintln(stdout)
	}

	return nil
}
