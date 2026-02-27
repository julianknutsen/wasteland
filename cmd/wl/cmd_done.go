package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newDoneCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		evidence string
		noPush   bool
	)

	cmd := &cobra.Command{
		Use:   "done <wanted-id>",
		Short: "Submit completion evidence for a wanted item",
		Long: `Submit completion evidence for a claimed wanted item.

Inserts a completion record and updates the wanted item status to 'in_review'.
The item must be claimed by your rig.

The --evidence flag provides the evidence URL (PR link, commit hash, etc.).

A completion ID is generated as c-<hash> where hash is derived from the
wanted ID, rig handle, and timestamp.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl done w-abc123 --evidence 'https://github.com/org/repo/pull/123'
  wl done w-abc123 --evidence 'commit abc123def'
  wl done w-abc123 --evidence 'commit abc123def' --no-push`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDone(cmd, stdout, stderr, args[0], evidence, noPush)
		},
	}

	cmd.Flags().StringVar(&evidence, "evidence", "", "Evidence URL or description (required)")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	_ = cmd.MarkFlagRequired("evidence")
	cmd.ValidArgsFunction = completeWantedIDs("claimed")

	return cmd
}

func runDone(cmd *cobra.Command, stdout, _ io.Writer, wantedID, evidence string, noPush bool) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	wantedID, err = resolveWantedArg(wlCfg, wantedID)
	if err != nil {
		return err
	}

	rigHandle := wlCfg.RigHandle

	mc := newMutationContext(wlCfg, wantedID, noPush, stdout)
	cleanup, err := mc.Setup()
	if err != nil {
		return err
	}
	defer cleanup()

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)
	completionID := commons.GeneratePrefixedID("c", wantedID, rigHandle)

	if err := submitDone(store, wantedID, rigHandle, evidence, completionID); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Completion submitted for %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Completion ID: %s\n", completionID)
	fmt.Fprintf(stdout, "  Completed by: %s\n", rigHandle)
	fmt.Fprintf(stdout, "  Evidence: %s\n", evidence)
	fmt.Fprintf(stdout, "  Status: in_review\n")
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: wait for review. Check: wl status "+wantedID))

	return nil
}

// submitDone contains the testable business logic for submitting a completion.
func submitDone(store commons.WLCommonsStore, wantedID, rigHandle, evidence, completionID string) error {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return fmt.Errorf("querying wanted item: %w", err)
	}

	if _, err := commons.ValidateTransition(item.Status, commons.TransitionDone); err != nil {
		return fmt.Errorf("wanted item %s: %w", wantedID, err)
	}

	if item.ClaimedBy != rigHandle {
		return fmt.Errorf("wanted item %s is claimed by %q, not %q", wantedID, item.ClaimedBy, rigHandle)
	}

	if err := store.SubmitCompletion(completionID, wantedID, rigHandle, evidence); err != nil {
		return fmt.Errorf("submitting completion: %w", err)
	}

	return nil
}
