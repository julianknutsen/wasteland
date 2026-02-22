package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
)

func newDoneCmd(stdout, stderr io.Writer) *cobra.Command {
	var evidence string

	cmd := &cobra.Command{
		Use:   "done <wanted-id>",
		Short: "Submit completion evidence for a wanted item",
		Long: `Submit completion evidence for a claimed wanted item.

Inserts a completion record and updates the wanted item status to 'in_review'.
The item must be claimed by your rig.

The --evidence flag provides the evidence URL (PR link, commit hash, etc.).

A completion ID is generated as c-<hash> where hash is derived from the
wanted ID, rig handle, and timestamp.

Examples:
  wl done w-abc123 --evidence 'https://github.com/org/repo/pull/123'
  wl done w-abc123 --evidence 'commit abc123def'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDone(cmd, stdout, stderr, args[0], evidence)
		},
	}

	cmd.Flags().StringVar(&evidence, "evidence", "", "Evidence URL or description (required)")
	_ = cmd.MarkFlagRequired("evidence")

	return cmd
}

func runDone(cmd *cobra.Command, stdout, stderr io.Writer, wantedID, evidence string) error {
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
	completionID := generateCompletionID(wantedID, rigHandle)

	if err := submitDone(store, wantedID, rigHandle, evidence, completionID); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Completion submitted for %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Completion ID: %s\n", completionID)
	fmt.Fprintf(stdout, "  Completed by: %s\n", rigHandle)
	fmt.Fprintf(stdout, "  Evidence: %s\n", evidence)
	fmt.Fprintf(stdout, "  Status: in_review\n")

	return nil
}

// submitDone contains the testable business logic for submitting a completion.
func submitDone(store commons.WLCommonsStore, wantedID, rigHandle, evidence, completionID string) error {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Status != "claimed" {
		return fmt.Errorf("wanted item %s is not claimed (status: %s)", wantedID, item.Status)
	}

	if item.ClaimedBy != rigHandle {
		return fmt.Errorf("wanted item %s is claimed by %q, not %q", wantedID, item.ClaimedBy, rigHandle)
	}

	if err := store.SubmitCompletion(completionID, wantedID, rigHandle, evidence); err != nil {
		return fmt.Errorf("submitting completion: %w", err)
	}

	return nil
}

func generateCompletionID(wantedID, rigHandle string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	h := sha256.Sum256([]byte(wantedID + "|" + rigHandle + "|" + now))
	return fmt.Sprintf("c-%x", h[:8])
}
