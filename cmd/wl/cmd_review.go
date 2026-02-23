package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/style"
)

func newReviewCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		jsonOut bool
		mdOut   bool
		statOut bool
	)

	cmd := &cobra.Command{
		Use:   "review [branch]",
		Short: "Review PR-mode branches",
		Long: `List or review PR-mode branches.

Without arguments, lists all wl/* branches.
With a branch name, shows the diff between main and the branch.

Output formats (mutually exclusive):
  (default)  Full diff piped to stdout
  --stat     Summary statistics
  --json     JSON diff output
  --md       Markdown-formatted diff for pasting into PRs

Examples:
  wl review                          # list wl/* branches
  wl review wl/my-rig/w-abc123       # terminal diff
  wl review wl/my-rig/w-abc123 --stat
  wl review wl/my-rig/w-abc123 --md`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var branch string
			if len(args) == 1 {
				branch = args[0]
			}
			return runReview(cmd, stdout, stderr, branch, jsonOut, mdOut, statOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output diff as JSON")
	cmd.Flags().BoolVar(&mdOut, "md", false, "Output diff as Markdown")
	cmd.Flags().BoolVar(&statOut, "stat", false, "Output diff statistics")

	return cmd
}

func runReview(cmd *cobra.Command, stdout, _ io.Writer, branch string, jsonOut, mdOut, statOut bool) error {
	// Validate mutually exclusive flags.
	flagCount := 0
	if jsonOut {
		flagCount++
	}
	if mdOut {
		flagCount++
	}
	if statOut {
		flagCount++
	}
	if flagCount > 1 {
		return fmt.Errorf("--json, --md, and --stat are mutually exclusive")
	}

	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	if branch == "" {
		return listReviewBranches(stdout, cfg.LocalDir)
	}

	doltPath, err := exec.LookPath("dolt")
	if err != nil {
		return fmt.Errorf("dolt not found in PATH — install from https://docs.dolthub.com/introduction/installation")
	}

	return showDiff(stdout, cfg.LocalDir, doltPath, branch, jsonOut, mdOut, statOut)
}

func listReviewBranches(stdout io.Writer, dbDir string) error {
	branches, err := commons.ListBranches(dbDir, "wl/")
	if err != nil {
		return fmt.Errorf("listing branches: %w", err)
	}

	if len(branches) == 0 {
		fmt.Fprintln(stdout, "No review branches found.")
		return nil
	}

	fmt.Fprintf(stdout, "%s\n", style.Bold.Render("Review branches:"))
	for _, b := range branches {
		fmt.Fprintf(stdout, "  %s\n", b)
	}
	return nil
}

func showDiff(stdout io.Writer, dbDir, doltPath, branch string, jsonOut, mdOut, statOut bool) error {
	if statOut {
		cmd := exec.Command(doltPath, "diff", "--stat", "main..."+branch)
		cmd.Dir = dbDir
		cmd.Stdout = stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("dolt diff --stat: %w", err)
		}
		return nil
	}

	if jsonOut {
		cmd := exec.Command(doltPath, "diff", "-r", "json", "main..."+branch)
		cmd.Dir = dbDir
		cmd.Stdout = stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("dolt diff -r json: %w", err)
		}
		return nil
	}

	if mdOut {
		return renderMarkdownDiff(stdout, dbDir, doltPath, branch)
	}

	// Default: full terminal diff.
	cmd := exec.Command(doltPath, "diff", "main..."+branch)
	cmd.Dir = dbDir
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dolt diff: %w", err)
	}
	return nil
}

func renderMarkdownDiff(stdout io.Writer, dbDir, doltPath, branch string) error {
	fmt.Fprintf(stdout, "## wl review: %s\n\n", branch)

	// Summary (stat).
	fmt.Fprintln(stdout, "### Summary")
	fmt.Fprintln(stdout, "```")

	statCmd := exec.Command(doltPath, "diff", "--stat", "main..."+branch)
	statCmd.Dir = dbDir
	statOut, err := statCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(stdout, "(no changes)\n")
	} else {
		fmt.Fprint(stdout, strings.TrimRight(string(statOut), "\n")+"\n")
	}
	fmt.Fprintln(stdout, "```")
	fmt.Fprintln(stdout)

	// Changes (SQL diff).
	fmt.Fprintln(stdout, "### Changes")
	fmt.Fprintln(stdout, "```sql")

	diffCmd := exec.Command(doltPath, "diff", "-r", "sql", "main..."+branch)
	diffCmd.Dir = dbDir
	diffOut, err := diffCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(stdout, "-- (no SQL changes)\n")
	} else {
		fmt.Fprint(stdout, strings.TrimRight(string(diffOut), "\n")+"\n")
	}
	fmt.Fprintln(stdout, "```")

	return nil
}
