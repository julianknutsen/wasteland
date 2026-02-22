package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
	"github.com/steveyegge/wasteland/internal/xdg"
)

func newSyncCmd(stdout, stderr io.Writer) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull upstream changes into local wl-commons fork",
		Args:  cobra.NoArgs,
		Long: `Sync your local wl-commons fork with the upstream hop/wl-commons.

If you have a local fork of wl-commons (created by wl join), this pulls
the latest changes from upstream.

EXAMPLES:
  wl sync                # Pull upstream changes
  wl sync --dry-run      # Show what would change`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(stdout, stderr, dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without pulling")

	return cmd
}

func runSync(stdout, stderr io.Writer, dryRun bool) error {
	doltPath, err := exec.LookPath("dolt")
	if err != nil {
		return fmt.Errorf("dolt not found in PATH — install from https://docs.dolthub.com/introduction/installation")
	}

	// Try loading wasteland config first (set by wl join)
	forkDir := ""
	if cfg, err := federation.LoadConfig(); err == nil {
		forkDir = cfg.LocalDir
	}

	// Fall back to standard locations
	if forkDir == "" {
		forkDir = findWLCommonsFork()
	}

	if forkDir == "" {
		return fmt.Errorf("no local wl-commons fork found\n\nJoin a wasteland first: wl join <org/db>")
	}

	fmt.Fprintf(stdout, "Local fork: %s\n", style.Dim.Render(forkDir))

	if dryRun {
		fmt.Fprintf(stdout, "\n%s Dry run — checking upstream for changes...\n", style.Bold.Render("~"))

		fetchCmd := exec.Command(doltPath, "fetch", "upstream")
		fetchCmd.Dir = forkDir
		fetchCmd.Stderr = os.Stderr
		if err := fetchCmd.Run(); err != nil {
			return fmt.Errorf("fetching upstream: %w", err)
		}

		diffCmd := exec.Command(doltPath, "diff", "--stat", "HEAD", "upstream/main")
		diffCmd.Dir = forkDir
		diffCmd.Stdout = os.Stdout
		diffCmd.Stderr = os.Stderr
		if err := diffCmd.Run(); err != nil {
			fmt.Fprintf(stdout, "%s Already up to date.\n", style.Bold.Render("✓"))
		}
		return nil
	}

	fmt.Fprintf(stdout, "\nPulling from upstream...\n")

	pullCmd := exec.Command(doltPath, "pull", "upstream", "main")
	pullCmd.Dir = forkDir
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("pulling from upstream: %w", err)
	}

	fmt.Fprintf(stdout, "\n%s Synced with upstream\n", style.Bold.Render("✓"))

	// Show summary
	summaryQuery := `SELECT
		(SELECT COUNT(*) FROM wanted WHERE status = 'open') AS open_wanted,
		(SELECT COUNT(*) FROM wanted) AS total_wanted,
		(SELECT COUNT(*) FROM completions) AS total_completions,
		(SELECT COUNT(*) FROM stamps) AS total_stamps`

	summaryCmd := exec.Command(doltPath, "sql", "-q", summaryQuery, "-r", "csv")
	summaryCmd.Dir = forkDir
	out, err := summaryCmd.Output()
	if err == nil {
		rows := wlParseCSV(string(out))
		if len(rows) >= 2 && len(rows[1]) >= 4 {
			r := rows[1]
			fmt.Fprintf(stdout, "\n  Open wanted:       %s\n", r[0])
			fmt.Fprintf(stdout, "  Total wanted:      %s\n", r[1])
			fmt.Fprintf(stdout, "  Total completions: %s\n", r[2])
			fmt.Fprintf(stdout, "  Total stamps:      %s\n", r[3])
		}
	}

	return nil
}

func findWLCommonsFork() string {
	dataDir := xdg.DataDir()
	candidates := []string{
		filepath.Join(dataDir, "wl-commons"),
		filepath.Join(os.Getenv("HOME"), "wl-commons"),
	}

	for _, dir := range candidates {
		doltDir := filepath.Join(dir, ".dolt")
		if info, err := os.Stat(doltDir); err == nil && info.IsDir() {
			return dir
		}
	}

	return ""
}
