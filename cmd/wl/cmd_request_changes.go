package main

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/style"
)

func newRequestChangesCmd(stdout, stderr io.Writer) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "request-changes <branch>",
		Short: "Request changes on a PR-mode branch",
		Long: `Submit a "request changes" review on the GitHub PR for a wl/* branch.

Requires a GitHub PR to exist (created via 'wl review --gh-pr').
The --comment flag is required to explain what needs to change.

Examples:
  wl request-changes wl/my-rig/w-abc123 --comment "needs tests"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRequestChanges(cmd, stdout, stderr, args[0], comment)
		},
	}

	cmd.Flags().StringVar(&comment, "comment", "", "Review comment (required)")
	_ = cmd.MarkFlagRequired("comment")

	return cmd
}

func runRequestChanges(cmd *cobra.Command, stdout, _ io.Writer, branch, comment string) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	if cfg.GitHubRepo == "" {
		return fmt.Errorf("github-repo not configured (run 'wl config set github-repo owner/repo')")
	}

	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh not found in PATH — install from https://cli.github.com")
	}

	client := newGHClient(ghPath)
	prURL, err := submitPRReview(client, cfg.GitHubRepo, cfg.ForkOrg, branch, "REQUEST_CHANGES", comment)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Requested changes on %s\n", style.Bold.Render("✓"), branch)
	fmt.Fprintf(stdout, "  %s\n", prURL)
	return nil
}
