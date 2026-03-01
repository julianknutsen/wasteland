package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/inference"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newInferRunCmd(stdout, stderr io.Writer) *cobra.Command {
	var noPush bool

	cmd := &cobra.Command{
		Use:   "run <wanted-id>",
		Short: "Claim and execute an inference job via ollama",
		Long: `Claim a wanted inference item and run it against the local ollama instance.

The item must have type=inference and status=open. The job parameters are
decoded from the description field. On success, the result (with SHA-256 hash)
is submitted as completion evidence. On failure, the claim is released so
another worker can retry.

Examples:
  wl infer run w-abc123
  wl infer run w-abc123 --no-push`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWantedIDs("open"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInferRun(cmd, stdout, stderr, args[0], noPush)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")

	return cmd
}

func runInferRun(cmd *cobra.Command, stdout, _ io.Writer, wantedID string, noPush bool) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
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

	completionID, err := executeInferRun(store, wantedID, wlCfg.RigHandle)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Inference completed for %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Completion ID: %s\n", completionID)
	fmt.Fprintf(stdout, "  Completed by:  %s\n", wlCfg.RigHandle)
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: wl infer verify "+wantedID))

	return nil
}

// executeInferRun is the testable business logic for running an inference job.
func executeInferRun(store commons.WLCommonsStore, wantedID, rigHandle string) (string, error) {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return "", fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Type != "inference" {
		return "", fmt.Errorf("wanted item %s has type %q, expected \"inference\"", wantedID, item.Type)
	}

	if item.Status != "open" {
		return "", fmt.Errorf("wanted item %s has status %q, expected \"open\"", wantedID, item.Status)
	}

	job, err := inference.DecodeJob(item.Description)
	if err != nil {
		return "", fmt.Errorf("decoding inference job from description: %w", err)
	}

	// Claim the item.
	if err := store.ClaimWanted(wantedID, rigHandle); err != nil {
		return "", fmt.Errorf("claiming wanted item: %w", err)
	}

	// Run inference.
	result, err := inference.Run(job)
	if err != nil {
		// Release claim so another worker can retry.
		_ = store.UnclaimWanted(wantedID)
		return "", fmt.Errorf("running inference: %w", err)
	}

	// Encode result as evidence.
	evidence, err := inference.EncodeResult(result)
	if err != nil {
		_ = store.UnclaimWanted(wantedID)
		return "", fmt.Errorf("encoding inference result: %w", err)
	}

	// Submit completion.
	completionID := commons.GeneratePrefixedID("c", wantedID, rigHandle)
	if err := store.SubmitCompletion(completionID, wantedID, rigHandle, evidence); err != nil {
		return "", fmt.Errorf("submitting completion: %w", err)
	}

	return completionID, nil
}
