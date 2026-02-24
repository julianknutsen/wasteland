package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newUnclaimCmd(stdout, stderr io.Writer) *cobra.Command {
	var noPush bool

	cmd := &cobra.Command{
		Use:   "unclaim <wanted-id>",
		Short: "Release a claimed wanted item back to open",
		Long: `Release a claimed wanted item, reverting it from 'claimed' to 'open'.

The item must be in 'claimed' status. Only the claimer or the poster can unclaim.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl unclaim w-abc123
  wl unclaim w-abc123 --no-push`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnclaim(cmd, stdout, stderr, args[0], noPush)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	cmd.ValidArgsFunction = completeWantedIDs("claimed")

	return cmd
}

func runUnclaim(cmd *cobra.Command, stdout, _ io.Writer, wantedID string, noPush bool) error {
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
	item, err := unclaimWanted(store, wantedID, rigHandle)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Unclaimed %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Title: %s\n", item.Title)
	fmt.Fprintf(stdout, "  Status: open\n")
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	return nil
}

// unclaimWanted contains the testable business logic for unclaiming a wanted item.
func unclaimWanted(store commons.WLCommonsStore, wantedID, rigHandle string) (*commons.WantedItem, error) {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Status != "claimed" {
		return nil, fmt.Errorf("wanted item %s is not claimed (status: %s)", wantedID, item.Status)
	}

	if item.ClaimedBy != rigHandle && item.PostedBy != rigHandle {
		return nil, fmt.Errorf("only the claimer or poster can unclaim (claimed by %q, posted by %q)", item.ClaimedBy, item.PostedBy)
	}

	if err := store.UnclaimWanted(wantedID); err != nil {
		return nil, fmt.Errorf("unclaiming wanted item: %w", err)
	}

	return item, nil
}
