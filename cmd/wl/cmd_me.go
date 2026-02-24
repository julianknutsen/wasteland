package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newMeCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show your personal dashboard",
		Long: `Show your personal dashboard: claimed items, items awaiting your review,
and recent completions.

Syncs with upstream first, then queries your local clone.

Examples:
  wl me`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMe(cmd, stdout, stderr)
		},
	}
}

func runMe(cmd *cobra.Command, stdout, _ io.Writer) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	if err := requireDolt(); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Syncing with upstream...\n")
	if err := commons.PullUpstream(cfg.LocalDir); err != nil {
		return fmt.Errorf("pulling upstream: %w", err)
	}

	handle := cfg.RigHandle
	printed := false

	// My claimed items
	claimedCSV, err := commons.DoltSQLQuery(cfg.LocalDir, fmt.Sprintf(
		"SELECT id, title, status, priority, effort_level FROM wanted WHERE claimed_by = '%s' AND status IN ('claimed','in_review') ORDER BY priority ASC",
		commons.EscapeSQL(handle),
	))
	if err != nil {
		return fmt.Errorf("querying claimed items: %w", err)
	}
	claimedRows := wlParseCSV(claimedCSV)
	if len(claimedRows) > 1 {
		fmt.Fprintf(stdout, "\n%s\n", style.Bold.Render("My claimed items:"))
		for _, row := range claimedRows[1:] {
			if len(row) < 5 {
				continue
			}
			pri := wlFormatPriority(row[3])
			fmt.Fprintf(stdout, "  %-12s %-30s %-12s %-4s %s\n", row[0], row[1], row[2], pri, row[4])
		}
		printed = true
	}

	// Awaiting my review
	reviewCSV, err := commons.DoltSQLQuery(cfg.LocalDir, fmt.Sprintf(
		"SELECT id, title, claimed_by FROM wanted WHERE posted_by = '%s' AND status = 'in_review' ORDER BY priority ASC",
		commons.EscapeSQL(handle),
	))
	if err != nil {
		return fmt.Errorf("querying review items: %w", err)
	}
	reviewRows := wlParseCSV(reviewCSV)
	if len(reviewRows) > 1 {
		fmt.Fprintf(stdout, "\n%s\n", style.Bold.Render("Awaiting my review:"))
		for _, row := range reviewRows[1:] {
			if len(row) < 3 {
				continue
			}
			fmt.Fprintf(stdout, "  %-12s %-30s claimed by %s\n", row[0], row[1], row[2])
		}
		printed = true
	}

	// Recent completions
	completedCSV, err := commons.DoltSQLQuery(cfg.LocalDir, fmt.Sprintf(
		"SELECT id, title FROM wanted WHERE claimed_by = '%s' AND status = 'completed' ORDER BY updated_at DESC LIMIT 5",
		commons.EscapeSQL(handle),
	))
	if err != nil {
		return fmt.Errorf("querying completed items: %w", err)
	}
	completedRows := wlParseCSV(completedCSV)
	if len(completedRows) > 1 {
		fmt.Fprintf(stdout, "\n%s\n", style.Bold.Render("Recent completions:"))
		for _, row := range completedRows[1:] {
			if len(row) < 2 {
				continue
			}
			fmt.Fprintf(stdout, "  %-12s %s\n", row[0], row[1])
		}
		printed = true
	}

	if !printed {
		fmt.Fprintf(stdout, "\nNothing here — browse the board: wl browse\n")
	}

	return nil
}
