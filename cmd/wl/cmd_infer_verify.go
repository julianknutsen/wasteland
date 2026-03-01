package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/inference"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newInferVerifyCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <wanted-id>",
		Short: "Re-run an inference job and compare hashes",
		Long: `Verify a completed inference job by re-running it and comparing output hashes.

The item must have type=inference and a submitted completion with evidence.
The job is re-run via ollama with the same parameters, and the output hash
is compared against the claimed result.

Examples:
  wl infer verify w-abc123`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWantedIDs("in_review"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInferVerify(cmd, stdout, stderr, args[0])
		},
	}

	return cmd
}

func runInferVerify(cmd *cobra.Command, stdout, _ io.Writer, wantedID string) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	wantedID, err = resolveWantedArg(wlCfg, wantedID)
	if err != nil {
		return err
	}

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	vr, err := executeInferVerify(store, wantedID)
	if err != nil {
		return err
	}

	if vr.Match {
		fmt.Fprintf(stdout, "%s VERIFIED — hashes match\n", style.Success.Render("✓"))
	} else {
		fmt.Fprintf(stdout, "%s MISMATCH — hashes differ\n", style.Error.Render("✗"))
	}
	fmt.Fprintf(stdout, "  Expected: %s\n", vr.ExpectedHash)
	fmt.Fprintf(stdout, "  Actual:   %s\n", vr.ActualHash)
	fmt.Fprintf(stdout, "  Output:   %s\n", truncate(vr.Output, 120))

	return nil
}

// executeInferVerify is the testable business logic for verifying an inference job.
func executeInferVerify(store commons.WLCommonsStore, wantedID string) (*inference.VerifyResult, error) {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Type != "inference" {
		return nil, fmt.Errorf("wanted item %s has type %q, expected \"inference\"", wantedID, item.Type)
	}

	job, err := inference.DecodeJob(item.Description)
	if err != nil {
		return nil, fmt.Errorf("decoding inference job: %w", err)
	}

	completion, err := store.QueryCompletion(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying completion: %w", err)
	}

	result, err := inference.DecodeResult(completion.Evidence)
	if err != nil {
		return nil, fmt.Errorf("decoding inference result from evidence: %w", err)
	}

	return inference.Verify(job, result)
}
