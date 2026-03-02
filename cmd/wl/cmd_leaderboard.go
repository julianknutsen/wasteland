package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/julianknutsen/wasteland/internal/backend"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newLeaderboardCmd(stdout, stderr io.Writer) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "leaderboard",
		Short: "Show the rig leaderboard",
		Long: `Show the Wasteland leaderboard — rigs ranked by validated completions.

Displays completion count, average quality and reliability scores,
and top skill tags for each rig that has earned at least one stamp.

EXAMPLES:
  wl leaderboard              # Top 20 rigs
  wl leaderboard --limit 10   # Top 10 rigs`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLeaderboard(cmd, stdout, stderr, limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of rigs to display")
	return cmd
}

func runLeaderboard(cmd *cobra.Command, stdout, _ io.Writer, limit int) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	if err := requireDolt(); err != nil {
		return err
	}

	db := backend.NewLocalDB(cfg.LocalDir, cfg.ResolveMode())

	sp := style.StartSpinner(stdout, "Syncing with upstream...")
	syncErr := db.Sync()
	sp.Stop()
	if syncErr != nil {
		return fmt.Errorf("syncing with upstream: %w", syncErr)
	}
	entries, err := commons.QueryLeaderboard(db, limit)
	if err != nil {
		return fmt.Errorf("querying leaderboard: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No validated completions yet — the leaderboard is empty.")
		return nil
	}

	tbl := style.NewTable(
		style.Column{Name: "#", Width: 4, Align: style.AlignRight},
		style.Column{Name: "RIG", Width: 20},
		style.Column{Name: "DONE", Width: 6, Align: style.AlignRight},
		style.Column{Name: "QUALITY", Width: 8, Align: style.AlignRight},
		style.Column{Name: "RELIAB", Width: 8, Align: style.AlignRight},
		style.Column{Name: "TOP SKILLS", Width: 30},
	)

	for i, e := range entries {
		rank := fmt.Sprintf("%d", i+1)
		done := fmt.Sprintf("%d", e.Completions)
		quality := fmt.Sprintf("%.1f", e.AvgQuality)
		reliab := fmt.Sprintf("%.1f", e.AvgReliab)
		skills := strings.Join(e.TopSkills, ", ")
		tbl.AddRow(rank, e.RigHandle, done, quality, reliab, skills)
	}

	fmt.Fprintf(stdout, "Leaderboard (%d rigs):\n\n", len(entries))
	fmt.Fprint(stdout, tbl.Render())

	return nil
}
