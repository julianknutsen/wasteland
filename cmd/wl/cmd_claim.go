package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
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
			return runClaim(cmd, stdout, stderr, args[0])
		},
	}
}

func runClaim(cmd *cobra.Command, stdout, stderr io.Writer, wantedID string) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}
	rigHandle := wlCfg.RigHandle

	org, db, _ := federation.ParseUpstream(wlCfg.Upstream)
	commonsDir := federation.WLCommonsDir(org, db)

	if _, err := os.Stat(filepath.Join(commonsDir, ".dolt")); err != nil {
		return fmt.Errorf("wl-commons database not found at %s\nRun 'wl post' first to initialize, or join a wasteland with: wl join <org/db>", commonsDir)
	}

	store := commons.NewWLCommons(commonsDir)
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
