package main

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/style"
)

func newApproveCmd(stdout, stderr io.Writer) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "approve <branch>",
		Short: "Approve a PR-mode branch",
		Long: `Submit an approving review on the GitHub PR for a wl/* branch.

Requires a GitHub PR to exist (created via 'wl review --gh-pr').

Examples:
  wl approve wl/my-rig/w-abc123
  wl approve wl/my-rig/w-abc123 --comment "LGTM"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApprove(cmd, stdout, stderr, args[0], comment)
		},
	}

	cmd.Flags().StringVar(&comment, "comment", "", "Review comment")

	return cmd
}

func runApprove(cmd *cobra.Command, stdout, _ io.Writer, branch, comment string) error {
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
	prURL, err := submitPRReview(client, cfg.GitHubRepo, cfg.ForkOrg, branch, "APPROVE", comment)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Approved %s\n", style.Bold.Render("✓"), branch)
	fmt.Fprintf(stdout, "  %s\n", prURL)
	return nil
}
