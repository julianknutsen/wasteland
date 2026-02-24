package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newCloseCmd(stdout, stderr io.Writer) *cobra.Command {
	var noPush bool

	cmd := &cobra.Command{
		Use:   "close <wanted-id>",
		Short: "Close an in_review item as completed (no stamp)",
		Long: `Close an in_review wanted item by marking it as completed without issuing
a reputation stamp. This is housekeeping for solo maintainers who posted,
claimed, and completed their own work.

The item must be in 'in_review' status and only the poster can close it.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl close w-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClose(cmd, stdout, stderr, args[0], noPush)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	cmd.ValidArgsFunction = completeWantedIDs("in_review")

	return cmd
}

func runClose(cmd *cobra.Command, stdout, _ io.Writer, wantedID string, noPush bool) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}
	rigHandle := wlCfg.RigHandle

	mc := newMutationContext(wlCfg, wantedID, noPush, stdout)
	cleanup, err := mc.Setup()
	if err != nil {
		return err
	}
	defer cleanup()

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	if err := closeWanted(store, wantedID, rigHandle); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Closed %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Status: completed\n")
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	return nil
}

// closeWanted contains the testable business logic for closing a wanted item.
func closeWanted(store commons.WLCommonsStore, wantedID, rigHandle string) error {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Status != "in_review" {
		return fmt.Errorf("wanted item %s is not in_review (status: %s)", wantedID, item.Status)
	}

	if item.PostedBy != rigHandle {
		return fmt.Errorf("only the poster can close (posted by %q)", item.PostedBy)
	}

	if err := store.CloseWanted(wantedID); err != nil {
		return fmt.Errorf("closing wanted item: %w", err)
	}

	return nil
}
