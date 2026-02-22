package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
	"github.com/steveyegge/wasteland/internal/xdg"
)

func newClaimCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "claim <wanted-id>",
		Short: "Claim a wanted item",
		Long: `Claim a wanted item on the shared wanted board.

Updates the wanted row: claimed_by=<your rig handle>, status='claimed'.
The item must exist and have status='open'.

In wild-west mode (Phase 1), this writes directly to the local wl-commons
database. In PR mode, this will create a DoltHub PR instead.

Examples:
  wl claim w-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClaim(stdout, stderr, args[0])
		},
	}
}

func runClaim(stdout, stderr io.Writer, wantedID string) error {
	wlCfg, err := federation.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}
	rigHandle := wlCfg.RigHandle

	dataDir := xdg.DataDir()
	if !commons.DatabaseExists(dataDir, commons.WLCommonsDB) {
		return fmt.Errorf("database %q not found\nJoin a wasteland first with: wl join <org/db>", commons.WLCommonsDB)
	}

	store := commons.NewWLCommons(dataDir)
	item, err := claimWanted(store, wantedID, rigHandle)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Claimed %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Claimed by: %s\n", rigHandle)
	fmt.Fprintf(stdout, "  Title: %s\n", item.Title)

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
