package main

import (
	"io"

	"github.com/spf13/cobra"
)

// newInferCmd creates the parent "wl infer" command group for verifiable inference.
func newInferCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "infer",
		Short: "Verifiable distributed LLM inference via ollama",
		Long: `Post, run, verify, and check status of LLM inference jobs.

Jobs are posted to the wanted board with type=inference. Workers claim and
run them via ollama with deterministic settings (temperature=0, fixed seed).
Results include a SHA-256 hash of the output for verification.

Commands:
  post    Post a new inference job to the wanted board
  run     Claim and execute an inference job via ollama
  verify  Re-run an inference job and compare hashes
  status  Show inference job details and results`,
	}

	cmd.AddCommand(
		newInferPostCmd(stdout, stderr),
		newInferRunCmd(stdout, stderr),
		newInferVerifyCmd(stdout, stderr),
		newInferStatusCmd(stdout, stderr),
	)

	return cmd
}
