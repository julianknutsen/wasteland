package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newDeleteCmd(stdout, stderr io.Writer) *cobra.Command {
	var noPush bool

	cmd := &cobra.Command{
		Use:   "delete <wanted-id>",
		Short: "Withdraw a wanted item",
		Long: `Withdraw a wanted item by setting its status to 'withdrawn'.

Only items with status 'open' can be withdrawn — claimed or in-review items
have active workers. The row stays in the table for audit trail.

In wild-west mode any joined rig can delete.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl delete w-abc123
  wl delete w-abc123 --no-push`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, stdout, stderr, args[0], noPush)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	cmd.ValidArgsFunction = completeWantedIDs("open")

	return cmd
}

func runDelete(cmd *cobra.Command, stdout, _ io.Writer, wantedID string, noPush bool) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	wantedID, err = resolveWantedArg(wlCfg, wantedID)
	if err != nil {
		return err
	}

	mc := newMutationContext(wlCfg, wantedID, noPush, stdout)
	cleanup, err := mc.Setup()
	if err != nil {
		return err
	}
	defer cleanup()

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	if err := deleteWanted(store, wantedID); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Withdrawn %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Status: withdrawn\n")
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	return nil
}

// deleteWanted contains the testable business logic for withdrawing a wanted item.
func deleteWanted(store commons.WLCommonsStore, wantedID string) error {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return fmt.Errorf("querying wanted item: %w", err)
	}

	if _, err := commons.ValidateTransition(item.Status, commons.TransitionDelete); err != nil {
		return fmt.Errorf("wanted item %s: %w", wantedID, err)
	}

	if err := store.DeleteWanted(wantedID); err != nil {
		return fmt.Errorf("deleting wanted item: %w", err)
	}

	return nil
}
