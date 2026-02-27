package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newRejectCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		reason string
		noPush bool
	)

	cmd := &cobra.Command{
		Use:   "reject <wanted-id>",
		Short: "Reject a completed wanted item back to claimed",
		Long: `Reject a completed wanted item, reverting it from 'in_review' to 'claimed'.

The item must be in 'in_review' status. Only the poster can reject.
The completion record is deleted so the claimer can re-submit.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl reject w-abc123
  wl reject w-abc123 --reason "tests failing"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReject(cmd, stdout, stderr, args[0], reason, noPush)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason for rejection (included in commit message)")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	cmd.ValidArgsFunction = completeWantedIDs("in_review")

	return cmd
}

func runReject(cmd *cobra.Command, stdout, _ io.Writer, wantedID, reason string, noPush bool) error {
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

	if err := rejectCompletion(store, wantedID, rigHandle, reason); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Rejected %s\n", style.Bold.Render("✓"), wantedID)
	if reason != "" {
		fmt.Fprintf(stdout, "  Reason: %s\n", reason)
	}
	fmt.Fprintf(stdout, "  Status: claimed\n")
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: claimer can fix and resubmit: wl done "+wantedID+" --evidence <url>"))

	return nil
}

// rejectCompletion contains the testable business logic for rejecting a completion.
func rejectCompletion(store commons.WLCommonsStore, wantedID, rigHandle, reason string) error {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return fmt.Errorf("querying wanted item: %w", err)
	}

	if _, err := commons.ValidateTransition(item.Status, commons.TransitionReject); err != nil {
		return fmt.Errorf("wanted item %s: %w", wantedID, err)
	}

	if item.PostedBy != rigHandle {
		return fmt.Errorf("only the poster can reject (posted by %q)", item.PostedBy)
	}

	if err := store.RejectCompletion(wantedID, rigHandle, reason); err != nil {
		return fmt.Errorf("rejecting completion: %w", err)
	}

	return nil
}
