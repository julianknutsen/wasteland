package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newBrowseCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		project   string
		status    string
		itemType  string
		priority  int
		limit     int
		jsonOut   bool
		ephemeral bool
		postedBy  string
		claimedBy string
		search    string
	)

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Browse wanted items on the commons board",
		Args:  cobra.NoArgs,
		Long: `Browse the Wasteland wanted board.

Pulls the latest upstream changes into your local clone and queries it.
Use --ephemeral to clone to a temp dir instead (slower, for edge cases).

EXAMPLES:
  wl browse                          # All open wanted items
  wl browse --project gastown        # Filter by project
  wl browse --type bug               # Only bugs
  wl browse --status claimed         # Claimed items
  wl browse --priority 0             # Critical priority only
  wl browse --limit 5               # Show 5 items
  wl browse --json                   # JSON output
  wl browse --posted-by alice        # Items posted by alice
  wl browse --claimed-by bob         # Items claimed by bob
  wl browse --search auth            # Search in title
  wl browse --ephemeral              # Clone upstream (slow)`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBrowse(cmd, stdout, stderr, commons.BrowseFilter{
				Status:    status,
				Project:   project,
				Type:      itemType,
				Priority:  priority,
				Limit:     limit,
				PostedBy:  postedBy,
				ClaimedBy: claimedBy,
				Search:    search,
			}, jsonOut, ephemeral)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Filter by project (e.g., gastown, beads, hop)")
	cmd.Flags().StringVar(&status, "status", "open", "Filter by status (open, claimed, in_review, completed, withdrawn)")
	cmd.Flags().StringVar(&itemType, "type", "", "Filter by type (feature, bug, design, rfc, docs)")
	cmd.Flags().IntVar(&priority, "priority", -1, "Filter by priority (0=critical, 2=medium, 4=backlog)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum items to display")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", false, "Clone upstream to temp dir instead of querying local (slow)")
	cmd.Flags().StringVar(&postedBy, "posted-by", "", "Filter by poster's rig handle")
	cmd.Flags().StringVar(&claimedBy, "claimed-by", "", "Filter by claimer's rig handle")
	cmd.Flags().StringVar(&search, "search", "", "Search in title")
	_ = cmd.RegisterFlagCompletionFunc("status", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"open", "claimed", "in_review", "completed", "withdrawn"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"feature", "bug", "design", "rfc", "docs"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runBrowse(cmd *cobra.Command, stdout, _ io.Writer, filter commons.BrowseFilter, jsonOut, ephemeral bool) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	if err := requireDolt(); err != nil {
		return err
	}

	query := commons.BuildBrowseQuery(filter)

	if ephemeral {
		return runBrowseEphemeral(stdout, cfg, query, jsonOut)
	}

	return runBrowseLocal(stdout, cfg, query, jsonOut)
}

func runBrowseLocal(stdout io.Writer, cfg *federation.Config, query string, jsonOut bool) error {
	sp := style.StartSpinner(stdout, "Syncing with upstream...")
	err := commons.PullUpstream(cfg.LocalDir)
	sp.Stop()
	if err != nil {
		return fmt.Errorf("pulling upstream: %w", err)
	}

	if jsonOut {
		doltPath, _ := exec.LookPath("dolt")
		sqlCmd := exec.Command(doltPath, "sql", "-q", query, "-r", "json")
		sqlCmd.Dir = cfg.LocalDir
		sqlCmd.Stdout = stdout
		sqlCmd.Stderr = os.Stderr
		return sqlCmd.Run()
	}

	csvData, err := commons.DoltSQLQuery(cfg.LocalDir, query)
	if err != nil {
		return fmt.Errorf("querying local database: %w", err)
	}

	return renderBrowseCSV(stdout, csvData)
}

func runBrowseEphemeral(stdout io.Writer, cfg *federation.Config, query string, jsonOut bool) error {
	doltPath, _ := exec.LookPath("dolt")

	_, commonsDB, _ := federation.ParseUpstream(cfg.Upstream)
	cloneURL := cfg.UpstreamURL
	if cloneURL == "" {
		cloneURL = cfg.Upstream
	}

	tmpDir, err := os.MkdirTemp("", "wl-browse-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cloneDir := filepath.Join(tmpDir, commonsDB)

	fmt.Fprintf(stdout, "Cloning %s...\n", style.Bold.Render(cfg.Upstream))

	cloneCmd := exec.Command(doltPath, "clone", cloneURL, cloneDir)
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("cloning %s: %w", cfg.Upstream, err)
	}
	fmt.Fprintf(stdout, "%s Cloned successfully\n\n", style.Bold.Render("✓"))

	if jsonOut {
		sqlCmd := exec.Command(doltPath, "sql", "-q", query, "-r", "json")
		sqlCmd.Dir = cloneDir
		sqlCmd.Stdout = stdout
		sqlCmd.Stderr = os.Stderr
		return sqlCmd.Run()
	}

	return renderBrowseTable(stdout, doltPath, cloneDir, query)
}

func renderBrowseCSV(stdout io.Writer, csvData string) error {
	rows := wlParseCSV(csvData)
	if len(rows) <= 1 {
		fmt.Fprintln(stdout, "No wanted items found matching your filters.")
		return nil
	}

	tbl := style.NewTable(
		style.Column{Name: "ID", Width: 12},
		style.Column{Name: "TITLE", Width: 40},
		style.Column{Name: "PROJECT", Width: 12},
		style.Column{Name: "TYPE", Width: 10},
		style.Column{Name: "PRI", Width: 4, Align: style.AlignRight},
		style.Column{Name: "POSTED BY", Width: 16},
		style.Column{Name: "STATUS", Width: 10},
		style.Column{Name: "EFFORT", Width: 8},
	)

	for _, row := range rows[1:] {
		if len(row) < 9 {
			continue
		}
		pri := wlFormatPriority(row[4])
		tbl.AddRow(row[0], row[1], row[2], row[3], pri, row[5], row[7], row[8])
	}

	fmt.Fprintf(stdout, "Wanted items (%d):\n\n", len(rows)-1)
	fmt.Fprint(stdout, tbl.Render())

	return nil
}

func renderBrowseTable(stdout io.Writer, doltPath, cloneDir, query string) error {
	sqlCmd := exec.Command(doltPath, "sql", "-q", query, "-r", "csv")
	sqlCmd.Dir = cloneDir
	output, err := sqlCmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("query failed: %s", string(exitErr.Stderr))
		}
		return fmt.Errorf("running query: %w", err)
	}

	return renderBrowseCSV(stdout, string(output))
}

func wlParseCSV(data string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		if line == "" {
			continue
		}
		rows = append(rows, wlParseCSVLine(line))
	}
	return rows
}

func wlParseCSVLine(line string) []string {
	var fields []string
	var field strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '"' && !inQuote:
			inQuote = true
		case ch == '"' && inQuote:
			if i+1 < len(line) && line[i+1] == '"' {
				field.WriteByte('"')
				i++
			} else {
				inQuote = false
			}
		case ch == ',' && !inQuote:
			fields = append(fields, field.String())
			field.Reset()
		default:
			field.WriteByte(ch)
		}
	}
	fields = append(fields, field.String())
	return fields
}

func wlFormatPriority(pri string) string {
	switch pri {
	case "0":
		return style.Error.Render("P0")
	case "1":
		return style.Warning.Render("P1")
	case "2":
		return "P2"
	case "3":
		return style.Dim.Render("P3")
	case "4":
		return style.Dim.Render("P4")
	default:
		return pri
	}
}
