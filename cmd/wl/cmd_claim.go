package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newClaimCmd(stdout, stderr io.Writer) *cobra.Command {
	var noPush bool

	cmd := &cobra.Command{
		Use:   "claim <wanted-id>",
		Short: "Claim a wanted item",
		Long: `Claim a wanted item on the shared wanted board.

Updates the wanted row: claimed_by=<your rig handle>, status='claimed'.
The item must exist and have status='open'.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl claim w-abc123
  wl claim w-abc123 --no-push`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClaim(cmd, stdout, stderr, args[0], noPush)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")

	return cmd
}

func runClaim(cmd *cobra.Command, stdout, _ io.Writer, wantedID string, noPush bool) error {
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
	item, err := claimWanted(store, wantedID, rigHandle)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Claimed %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Claimed by: %s\n", rigHandle)
	fmt.Fprintf(stdout, "  Title: %s\n", item.Title)
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	mc.Push()

	return nil
}

// claimWanted contains the testable business logic for claiming a wanted item.
func claimWanted(store commons.WLCommonsStore, wantedID, rigHandle string) (*commons.WantedItem, error) {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Status != "open" {
		return nil, fmt.Errorf("wanted item %s is not open (status: %s)", wantedID, item.Status)
	}

	if err := store.ClaimWanted(wantedID, rigHandle); err != nil {
		return nil, fmt.Errorf("claiming wanted item: %w", err)
	}

	return item, nil
}
