package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
)

func newLeaveCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "leave [upstream]",
		Short: "Leave a wasteland",
		Long: `Leave a wasteland by removing its configuration.

If only one wasteland is joined, no argument is needed.
If multiple are joined, specify the upstream or use --wasteland.

Local data directories (fork clone, wl_commons database) are NOT deleted
automatically. The command prints their paths for manual cleanup.

Examples:
  wl leave
  wl leave steveyegge/wl-commons
  wl leave --wasteland steveyegge/wl-commons`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var positional string
			if len(args) > 0 {
				positional = args[0]
			}
			return runLeave(cmd, stdout, stderr, positional)
		},
	}
}

func runLeave(cmd *cobra.Command, stdout, stderr io.Writer, positional string) error {
	store := federation.NewConfigStore()

	// Determine which upstream to leave: positional arg > --wasteland flag > auto.
	explicit := positional
	if explicit == "" {
		explicit, _ = cmd.Flags().GetString("wasteland")
	}

	cfg, err := federation.ResolveConfig(store, explicit)
	if err != nil {
		return fmt.Errorf("resolving wasteland: %w", err)
	}

	upstream := cfg.Upstream
	if err := store.Delete(upstream); err != nil {
		return fmt.Errorf("removing wasteland config: %w", err)
	}

	org, db, _ := federation.ParseUpstream(upstream)

	fmt.Fprintf(stdout, "%s Left wasteland: %s\n", style.Bold.Render("✓"), upstream)
	fmt.Fprintf(stdout, "\n  Data directories (not deleted):\n")
	fmt.Fprintf(stdout, "    Fork clone: %s\n", cfg.LocalDir)
	fmt.Fprintf(stdout, "    WL commons: %s\n", federation.WLCommonsDir(org, db))
	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Remove these directories manually if no longer needed."))

	return nil
}
