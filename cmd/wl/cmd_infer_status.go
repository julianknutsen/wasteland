package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/inference"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newInferStatusCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <wanted-id>",
		Short: "Show inference job details and results",
		Long: `Show detailed status for an inference wanted item.

Displays the standard wanted item fields plus decoded inference metadata
(prompt, model, seed). If the item has a completion, shows the output hash
and a truncated output preview.

Examples:
  wl infer status w-abc123`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWantedIDs(""),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInferStatus(cmd, stdout, stderr, args[0])
		},
	}

	return cmd
}

func runInferStatus(cmd *cobra.Command, stdout, _ io.Writer, wantedID string) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	wantedID, err = resolveWantedArg(wlCfg, wantedID)
	if err != nil {
		return err
	}

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	result, err := getStatus(store, wantedID)
	if err != nil {
		return err
	}

	renderInferStatus(stdout, result)
	return nil
}

// renderInferStatus writes inference-specific status output.
func renderInferStatus(w io.Writer, r *StatusResult) {
	item := r.Item

	// Standard header.
	fmt.Fprintf(w, "%s\n", style.Bold.Render(fmt.Sprintf("%s: %s", item.ID, item.Title)))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Status: %s\n", colorizeStatus(item.Status))
	fmt.Fprintf(w, "  Type:   %s\n", item.Type)

	// Decode and display inference job metadata.
	if job, err := inference.DecodeJob(item.Description); err == nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Inference Job:")
		fmt.Fprintf(w, "    Model:      %s\n", job.Model)
		fmt.Fprintf(w, "    Seed:       %d\n", job.Seed)
		if job.MaxTokens > 0 {
			fmt.Fprintf(w, "    Max tokens: %d\n", job.MaxTokens)
		}
		fmt.Fprintf(w, "    Prompt:     %s\n", truncate(job.Prompt, 100))
	} else {
		fmt.Fprintf(w, "\n  Description: %s\n", truncate(item.Description, 100))
	}

	if item.PostedBy != "" {
		fmt.Fprintf(w, "\n  Posted by: %s\n", item.PostedBy)
	}
	if item.ClaimedBy != "" {
		fmt.Fprintf(w, "  Claimed by: %s\n", item.ClaimedBy)
	}

	// Show completion details if available.
	if r.Completion != nil {
		if result, err := inference.DecodeResult(r.Completion.Evidence); err == nil {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  Inference Result:")
			fmt.Fprintf(w, "    Hash:   %s\n", result.OutputHash)
			fmt.Fprintf(w, "    Output: %s\n", truncate(result.Output, 120))
		}
	}
}
