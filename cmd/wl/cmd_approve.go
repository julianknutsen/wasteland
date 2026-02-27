package main

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newApproveCmd(stdout, stderr io.Writer) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "approve <branch>",
		Short: "Approve a PR-mode branch",
		Long: `Submit an approving review on the GitHub PR for a wl/* branch.

Requires a GitHub PR to exist (created via 'wl review --create-pr').

Examples:
  wl approve wl/my-rig/w-abc123
  wl approve wl/my-rig/w-abc123 --comment "LGTM"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApprove(cmd, stdout, stderr, args[0], comment)
		},
	}

	cmd.Flags().StringVar(&comment, "comment", "", "Review comment")
	cmd.ValidArgsFunction = completeBranchNames

	return cmd
}

func runApprove(cmd *cobra.Command, stdout, _ io.Writer, branch, comment string) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	if !cfg.IsGitHub() {
		return fmt.Errorf("approve requires GitHub provider (joined with --github)")
	}

	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh not found in PATH — install from https://cli.github.com")
	}

	client := newGHClient(ghPath)
	prURL, err := submitPRReview(client, cfg.Upstream, cfg.ForkOrg, branch, "APPROVE", comment)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Approved %s\n", style.Bold.Render("✓"), branch)
	fmt.Fprintf(stdout, "  %s\n", prURL)
	return nil
}
