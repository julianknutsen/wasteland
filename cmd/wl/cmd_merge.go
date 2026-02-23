package main

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/style"
)

func newMergeCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		noPush     bool
		keepBranch bool
	)

	cmd := &cobra.Command{
		Use:   "merge <branch>",
		Short: "Merge a reviewed branch into main",
		Long: `Merge a wl/* branch into main after review.

Performs a Dolt merge, pushes main to upstream and origin, and deletes
the branch (unless --keep-branch is set).

Examples:
  wl merge wl/my-rig/w-abc123
  wl merge wl/my-rig/w-abc123 --keep-branch
  wl merge wl/my-rig/w-abc123 --no-push`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMerge(cmd, stdout, stderr, args[0], noPush, keepBranch)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes")
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "Don't delete branch after merge")

	return cmd
}

func runMerge(cmd *cobra.Command, stdout, _ io.Writer, branch string, noPush, keepBranch bool) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	exists, err := commons.BranchExists(cfg.LocalDir, branch)
	if err != nil {
		return fmt.Errorf("checking branch: %w", err)
	}
	if !exists {
		return fmt.Errorf("branch %q does not exist", branch)
	}

	// Best-effort: check PR approval status before merging.
	if cfg.GitHubRepo != "" {
		if ghPath, err := exec.LookPath("gh"); err == nil {
			hasApproval, hasChangesRequested := prApprovalStatus(ghPath, cfg.GitHubRepo, cfg.ForkOrg, branch)
			if hasChangesRequested {
				fmt.Fprintf(stdout, "  %s PR has outstanding change requests\n", style.Warning.Render("⚠"))
			} else if !hasApproval {
				fmt.Fprintf(stdout, "  %s PR has no approvals\n", style.Warning.Render("⚠"))
			}
		}
	}

	if err := commons.MergeBranch(cfg.LocalDir, branch); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Merged %s into main\n", style.Bold.Render("✓"), branch)

	if !keepBranch {
		if err := commons.DeleteBranch(cfg.LocalDir, branch); err != nil {
			fmt.Fprintf(stdout, "  warning: failed to delete branch %s: %v\n", branch, err)
		} else {
			fmt.Fprintf(stdout, "  Branch %s deleted\n", branch)
		}
	}

	if !noPush {
		_ = commons.PushWithSync(cfg.LocalDir, stdout)
	}

	// Best-effort: auto-close the corresponding GitHub PR shell.
	if cfg.GitHubRepo != "" {
		if ghPath, err := exec.LookPath("gh"); err == nil {
			closeGitHubPR(ghPath, cfg.GitHubRepo, cfg.ForkOrg, cfg.ForkDB, branch, stdout)
		}
	}

	return nil
}
