package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newMeCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show your personal dashboard",
		Long: `Show your personal dashboard: claimed items, items awaiting your review,
in-flight branch work (PR mode), and recent completions.

Fetches both upstream (master DB) and origin (your fork), then shows
where each item lives. Also scans wl/<handle>/* branches for PR-mode
items not yet on main.

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

	handle := cfg.RigHandle
	dbDir := cfg.LocalDir

	// Fetch both remotes (non-destructive — no merge into local main).
	sp := style.StartSpinner(stdout, "Fetching upstream and origin...")
	upstreamOK := commons.FetchRemote(dbDir, "upstream") == nil
	originOK := commons.FetchRemote(dbDir, "origin") == nil
	sp.Stop()

	printed := false

	// Items on upstream/main (the canonical master DB).
	if upstreamOK {
		upstreamItems := queryClaimedAsOf(dbDir, handle, "upstream/main")
		if len(upstreamItems) > 0 {
			fmt.Fprintf(stdout, "\n%s\n", style.Bold.Render("On upstream (master DB):"))
			for _, row := range upstreamItems {
				fmt.Fprintf(stdout, "  %-12s %-30s %-12s %-4s %-8s%s\n", row[0], row[1], row[2], wlFormatPriority(row[3]), row[4], staleWarning(row[5]))
			}
			printed = true
		}
	}

	// Items on origin/main but NOT on upstream/main (fork only).
	if originOK {
		upstreamIDs := map[string]bool{}
		if upstreamOK {
			for _, row := range queryClaimedAsOf(dbDir, handle, "upstream/main") {
				upstreamIDs[row[0]] = true
			}
		}
		originItems := queryClaimedAsOf(dbDir, handle, "origin/main")
		var forkOnly [][]string
		for _, row := range originItems {
			if !upstreamIDs[row[0]] {
				forkOnly = append(forkOnly, row)
			}
		}
		if len(forkOnly) > 0 {
			fmt.Fprintf(stdout, "\n%s\n", style.Bold.Render("On origin only (your fork — not yet on upstream):"))
			for _, row := range forkOnly {
				fmt.Fprintf(stdout, "  %-12s %-30s %-12s %-4s %-8s%s\n", row[0], row[1], row[2], wlFormatPriority(row[3]), row[4], staleWarning(row[5]))
			}
			printed = true
		}
	}

	// In-flight branches (PR-mode items on wl/<handle>/* branches).
	seenIDs := map[string]bool{}
	if upstreamOK {
		for _, row := range queryClaimedAsOf(dbDir, handle, "upstream/main") {
			seenIDs[row[0]] = true
		}
	}
	if originOK {
		for _, row := range queryClaimedAsOf(dbDir, handle, "origin/main") {
			seenIDs[row[0]] = true
		}
	}
	branchItems := listBranchItems(dbDir, handle, seenIDs)
	if len(branchItems) > 0 {
		// Group by location so each section header is unambiguous.
		groups := []struct {
			location string
			header   string
		}{
			{"origin + upstream", "On branches (origin + upstream, not merged to main):"},
			{"upstream", "On branches (upstream, not merged to main):"},
			{"origin", "On branches (origin only — not on upstream):"},
			{"local only", "On local branches only (not pushed):"},
		}
		for _, g := range groups {
			var matched []branchItem
			for _, bi := range branchItems {
				if bi.location == g.location {
					matched = append(matched, bi)
				}
			}
			if len(matched) == 0 {
				continue
			}
			fmt.Fprintf(stdout, "\n%s\n", style.Bold.Render(g.header))
			for _, bi := range matched {
				fmt.Fprintf(stdout, "  %-12s %-30s %s\n", bi.id, bi.title, style.Dim.Render(bi.branch))
			}
			printed = true
		}
	}

	// Awaiting my review (on upstream, the canonical source).
	ref := "upstream/main"
	if !upstreamOK {
		ref = "main"
	}
	reviewCSV, err := commons.DoltSQLQuery(dbDir, fmt.Sprintf(
		"SELECT id, title, claimed_by FROM wanted AS OF '%s' WHERE posted_by = '%s' AND status = 'in_review' ORDER BY priority ASC",
		commons.EscapeSQL(ref), commons.EscapeSQL(handle),
	))
	if err == nil {
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
	}

	// Recent completions (on upstream).
	completedCSV, err := commons.DoltSQLQuery(dbDir, fmt.Sprintf(
		"SELECT id, title FROM wanted AS OF '%s' WHERE claimed_by = '%s' AND status = 'completed' ORDER BY updated_at DESC LIMIT 5",
		commons.EscapeSQL(ref), commons.EscapeSQL(handle),
	))
	if err == nil {
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
	}

	if !printed {
		fmt.Fprintf(stdout, "\nNothing here — browse the board: wl browse\n")
	}

	return nil
}

// queryClaimedAsOf queries claimed/in_review items for a handle on a specific ref.
// Returns data rows (no header) with columns: id, title, status, priority, effort_level, days_stale.
func queryClaimedAsOf(dbDir, handle, ref string) [][]string {
	csv, err := commons.DoltSQLQuery(dbDir, fmt.Sprintf(
		"SELECT id, title, status, priority, effort_level, DATEDIFF(NOW(), updated_at) AS days_stale FROM wanted AS OF '%s' WHERE claimed_by = '%s' AND status IN ('claimed','in_review') ORDER BY priority ASC",
		commons.EscapeSQL(ref), commons.EscapeSQL(handle),
	))
	if err != nil {
		return nil
	}
	rows := wlParseCSV(csv)
	if len(rows) <= 1 {
		return nil
	}
	var data [][]string
	for _, row := range rows[1:] {
		if len(row) >= 6 {
			data = append(data, row)
		}
	}
	return data
}

// staleWarning returns a dimmed warning if the item has been claimed for more than 7 days.
func staleWarning(daysStr string) string {
	days, err := strconv.Atoi(strings.TrimSpace(daysStr))
	if err != nil || days <= 7 {
		return ""
	}
	return style.Dim.Render(fmt.Sprintf("  (%dd — consider: wl done or wl unclaim)", days))
}

// branchItem represents a wanted item found on a wl/* branch.
type branchItem struct {
	id       string
	title    string
	branch   string
	location string // "local only", "origin", "upstream", "origin + upstream"
}

// remoteBranchSet returns the set of branch names that exist on remotes,
// keyed by the local branch name (e.g. "wl/alice/w-123" → {"origin": true}).
func remoteBranchSet(dbDir string) map[string]map[string]bool {
	csv, err := commons.DoltSQLQuery(dbDir, "SELECT name FROM dolt_remote_branches")
	if err != nil {
		return nil
	}
	rows := wlParseCSV(csv)
	result := make(map[string]map[string]bool)
	for i, row := range rows {
		if i == 0 || len(row) < 1 { // skip header
			continue
		}
		// name is "remotes/origin/wl/alice/w-123" or "remotes/upstream/main"
		name := row[0]
		name = strings.TrimPrefix(name, "remotes/")
		slashIdx := strings.Index(name, "/")
		if slashIdx < 0 {
			continue
		}
		remote := name[:slashIdx]
		localName := name[slashIdx+1:]
		if result[localName] == nil {
			result[localName] = make(map[string]bool)
		}
		result[localName][remote] = true
	}
	return result
}

// branchLocation returns a human-readable location string for a branch.
func branchLocation(remotes map[string]map[string]bool, branch string) string {
	if remotes == nil {
		return "local only"
	}
	rm := remotes[branch]
	onOrigin := rm["origin"]
	onUpstream := rm["upstream"]
	switch {
	case onOrigin && onUpstream:
		return "origin + upstream"
	case onOrigin:
		return "origin"
	case onUpstream:
		return "upstream"
	default:
		return "local only"
	}
}

// listBranchItems scans wl/<handle>/* branches and returns items that differ
// from main — i.e., PR-mode work not yet merged.
func listBranchItems(dbDir, handle string, skipIDs map[string]bool) []branchItem {
	prefix := fmt.Sprintf("wl/%s/", handle)
	branches, err := commons.ListBranches(dbDir, prefix)
	if err != nil || len(branches) == 0 {
		return nil
	}

	remotes := remoteBranchSet(dbDir)

	var items []branchItem
	for _, branch := range branches {
		wantedID := extractBranchWantedID(branch)
		if wantedID == "" || skipIDs[wantedID] {
			continue
		}
		// Query the wanted table AS OF this branch to get the item state.
		query := fmt.Sprintf(
			"SELECT id, title, status FROM wanted AS OF '%s' WHERE id = '%s' LIMIT 1",
			commons.EscapeSQL(branch), commons.EscapeSQL(wantedID),
		)
		csv, err := commons.DoltSQLQuery(dbDir, query)
		if err != nil {
			continue
		}
		rows := wlParseCSV(csv)
		if len(rows) < 2 || len(rows[1]) < 3 {
			continue
		}
		items = append(items, branchItem{
			id:       rows[1][0],
			title:    rows[1][1],
			branch:   branch,
			location: branchLocation(remotes, branch),
		})
	}
	return items
}

// extractBranchWantedID extracts the wanted ID from wl/<rig>/<id>.
func extractBranchWantedID(branch string) string {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
