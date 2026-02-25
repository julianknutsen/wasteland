package commons

import (
	"fmt"
	"strings"
)

// SortOrder defines browse result ordering.
type SortOrder int

// Sort order constants for browse results.
const (
	SortPriority SortOrder = iota
	SortNewest
	SortAlpha
)

// ValidSortOrders returns all sort modes.
func ValidSortOrders() []SortOrder {
	return []SortOrder{SortPriority, SortNewest, SortAlpha}
}

// SortLabel returns a human-readable label for a sort order.
func SortLabel(s SortOrder) string {
	switch s {
	case SortPriority:
		return "priority"
	case SortNewest:
		return "newest"
	case SortAlpha:
		return "alpha"
	default:
		return "priority"
	}
}

// BrowseFilter holds filter parameters for querying the wanted board.
type BrowseFilter struct {
	Status    string
	Project   string
	Type      string
	Priority  int // -1 means unset
	Limit     int
	PostedBy  string
	ClaimedBy string
	Search    string
	MyItems   string    // rig handle for OR filter (posted_by OR claimed_by); empty = disabled
	Sort      SortOrder // result ordering
}

// WantedSummary holds the columns returned by BrowseWanted.
type WantedSummary struct {
	ID          string
	Title       string
	Project     string
	Type        string
	Priority    int
	PostedBy    string
	ClaimedBy   string
	Status      string
	EffortLevel string
}

// BrowseWanted queries the wanted board with the given filters.
// Returns structured results suitable for both CLI and TUI rendering.
func BrowseWanted(dbDir string, f BrowseFilter) ([]WantedSummary, error) {
	query := BuildBrowseQuery(f)
	csvData, err := DoltSQLQuery(dbDir, query)
	if err != nil {
		return nil, fmt.Errorf("querying wanted board: %w", err)
	}
	return parseWantedSummaries(csvData), nil
}

// BuildBrowseQuery builds a SQL query from a BrowseFilter.
func BuildBrowseQuery(f BrowseFilter) string {
	var conditions []string

	if f.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = '%s'", EscapeSQL(f.Status)))
	}
	if f.Project != "" {
		conditions = append(conditions, fmt.Sprintf("project = '%s'", EscapeSQL(f.Project)))
	}
	if f.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = '%s'", EscapeSQL(f.Type)))
	}
	if f.Priority >= 0 {
		conditions = append(conditions, fmt.Sprintf("priority = %d", f.Priority))
	}
	if f.MyItems != "" {
		escaped := EscapeSQL(f.MyItems)
		conditions = append(conditions, fmt.Sprintf("(posted_by = '%s' OR claimed_by = '%s')", escaped, escaped))
	} else {
		if f.PostedBy != "" {
			conditions = append(conditions, fmt.Sprintf("posted_by = '%s'", EscapeSQL(f.PostedBy)))
		}
		if f.ClaimedBy != "" {
			conditions = append(conditions, fmt.Sprintf("claimed_by = '%s'", EscapeSQL(f.ClaimedBy)))
		}
	}
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("title LIKE '%%%s%%'", EscapeSQL(f.Search)))
	}

	query := "SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	switch f.Sort {
	case SortNewest:
		query += " ORDER BY created_at DESC"
	case SortAlpha:
		query += " ORDER BY title ASC"
	default:
		query += " ORDER BY priority ASC, created_at DESC"
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	return query
}

// BranchOverride maps a wanted ID to its status on a local mutation branch.
type BranchOverride struct {
	WantedID string
	Branch   string
	Status   string
}

// DetectBranchOverrides lists local wl/<rigHandle>/* branches and queries
// each item's status via AS OF. Returns overrides for items whose branch
// status differs from their main status.
func DetectBranchOverrides(dbDir, rigHandle string) []BranchOverride {
	prefix := fmt.Sprintf("wl/%s/", rigHandle)
	branches, err := ListBranches(dbDir, prefix)
	if err != nil || len(branches) == 0 {
		return nil
	}

	var overrides []BranchOverride
	for _, branch := range branches {
		wantedID := strings.TrimPrefix(branch, prefix)
		branchStatus := QueryItemStatusAsOf(dbDir, wantedID, branch)
		if branchStatus == "" {
			continue
		}
		mainStatus := QueryItemStatusAsOf(dbDir, wantedID, "")
		if branchStatus != mainStatus {
			overrides = append(overrides, BranchOverride{
				WantedID: wantedID,
				Branch:   branch,
				Status:   branchStatus,
			})
		}
	}
	return overrides
}

// ApplyBranchOverrides adjusts browse results to reflect branch mutations.
// Items with overrides get their status updated. Items excluded by the status
// filter but matching after override are fetched from main and added.
// Items included but no longer matching after override are removed.
func ApplyBranchOverrides(dbDir string, items []WantedSummary, overrides []BranchOverride, statusFilter string) []WantedSummary {
	if len(overrides) == 0 {
		return items
	}

	byID := make(map[string]BranchOverride, len(overrides))
	for _, o := range overrides {
		byID[o.WantedID] = o
	}

	applied := make(map[string]bool)
	var result []WantedSummary
	for _, item := range items {
		if o, ok := byID[item.ID]; ok {
			applied[item.ID] = true
			item.Status = o.Status
			if statusFilter != "" && item.Status != statusFilter {
				continue // override made it not match the filter
			}
		}
		result = append(result, item)
	}

	// Add items that weren't in the main results but now match the filter.
	for _, o := range overrides {
		if applied[o.WantedID] {
			continue
		}
		if statusFilter != "" && o.Status != statusFilter {
			continue
		}
		// Fetch summary from main (metadata is the same) and override status.
		if item, err := QueryWantedDetail(dbDir, o.WantedID); err == nil {
			result = append(result, WantedSummary{
				ID:          item.ID,
				Title:       item.Title,
				Project:     item.Project,
				Type:        item.Type,
				Priority:    item.Priority,
				PostedBy:    item.PostedBy,
				ClaimedBy:   item.ClaimedBy,
				Status:      o.Status,
				EffortLevel: item.EffortLevel,
			})
		}
	}

	return result
}

// FindBranchForItem returns the branch name if a mutation branch exists for
// this item, or "" if not.
func FindBranchForItem(dbDir, rigHandle, wantedID string) string {
	branch := BranchName(rigHandle, wantedID)
	if exists, _ := BranchExists(dbDir, branch); exists {
		return branch
	}
	return ""
}

// ValidStatuses returns the browse filter status cycle.
func ValidStatuses() []string {
	return []string{"open", "claimed", "in_review", "completed", ""}
}

// ValidTypes returns the browse filter type cycle.
func ValidTypes() []string {
	return []string{"", "feature", "bug", "design", "rfc", "docs"}
}

// StatusLabel returns a human-readable label for a status filter value.
func StatusLabel(status string) string {
	if status == "" {
		return "all"
	}
	return status
}

// TypeLabel returns a human-readable label for a type filter value.
func TypeLabel(typ string) string {
	if typ == "" {
		return "all"
	}
	return typ
}

// ValidPriorities returns the browse filter priority cycle values.
// -1 means "all" (unfiltered).
func ValidPriorities() []int {
	return []int{-1, 0, 1, 2, 3, 4}
}

// PriorityLabel returns a human-readable label for a priority filter value.
func PriorityLabel(pri int) string {
	if pri < 0 {
		return "all"
	}
	return fmt.Sprintf("P%d", pri)
}

// DashboardData holds the sections for the "me" dashboard view.
type DashboardData struct {
	Claimed   []WantedSummary // status=claimed, claimed_by=me
	InReview  []WantedSummary // status=in_review, posted_by=me
	Completed []WantedSummary // status=completed, claimed_by=me, limit 5
}

// QueryMyDashboard fetches personal dashboard data for the given handle.
func QueryMyDashboard(dbDir, handle string) (*DashboardData, error) {
	escaped := EscapeSQL(handle)
	data := &DashboardData{}

	// Claimed items.
	claimedQ := fmt.Sprintf(
		"SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted WHERE status = 'claimed' AND claimed_by = '%s' ORDER BY priority ASC, created_at DESC LIMIT 50",
		escaped)
	csv, err := DoltSQLQuery(dbDir, claimedQ)
	if err != nil {
		return nil, fmt.Errorf("dashboard claimed: %w", err)
	}
	data.Claimed = parseWantedSummaries(csv)

	// In-review items (posted by me, awaiting my review).
	reviewQ := fmt.Sprintf(
		"SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted WHERE status = 'in_review' AND posted_by = '%s' ORDER BY priority ASC, created_at DESC LIMIT 50",
		escaped)
	csv, err = DoltSQLQuery(dbDir, reviewQ)
	if err != nil {
		return nil, fmt.Errorf("dashboard in_review: %w", err)
	}
	data.InReview = parseWantedSummaries(csv)

	// Recent completions.
	completedQ := fmt.Sprintf(
		"SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted WHERE status = 'completed' AND claimed_by = '%s' ORDER BY created_at DESC LIMIT 5",
		escaped)
	csv, err = DoltSQLQuery(dbDir, completedQ)
	if err != nil {
		return nil, fmt.Errorf("dashboard completed: %w", err)
	}
	data.Completed = parseWantedSummaries(csv)

	return data, nil
}

// BrowseWantedBranchAware wraps BrowseWanted with branch overlay in PR mode.
// Returns the items and a map of wanted IDs that have active branches.
func BrowseWantedBranchAware(dbDir, mode, rigHandle string, f BrowseFilter) ([]WantedSummary, map[string]bool, error) {
	items, err := BrowseWanted(dbDir, f)
	if err != nil {
		return nil, nil, err
	}
	branchIDs := make(map[string]bool)
	if mode == "pr" {
		overrides := DetectBranchOverrides(dbDir, rigHandle)
		for _, o := range overrides {
			branchIDs[o.WantedID] = true
		}
		items = ApplyBranchOverrides(dbDir, items, overrides, f.Status)
	}
	return items, branchIDs, nil
}

// QueryFullDetail fetches a wanted item with all related records (completion, stamp).
func QueryFullDetail(dbDir, wantedID string) (*WantedItem, *CompletionRecord, *Stamp, error) {
	item, err := QueryWantedDetail(dbDir, wantedID)
	if err != nil {
		return nil, nil, nil, err
	}

	var completion *CompletionRecord
	var stamp *Stamp

	if item.Status == "in_review" || item.Status == "completed" {
		if c, err := QueryCompletion(dbDir, wantedID); err == nil {
			completion = c
			if c.StampID != "" {
				if s, err := QueryStamp(dbDir, c.StampID); err == nil {
					stamp = s
				}
			}
		}
	}

	return item, completion, stamp, nil
}

// parseWantedSummaries parses CSV output into WantedSummary structs.
func parseWantedSummaries(csvData string) []WantedSummary {
	rows := parseSimpleCSV(csvData)
	var results []WantedSummary
	for _, row := range rows {
		pri := 2
		if v, ok := row["priority"]; ok {
			_, _ = fmt.Sscanf(v, "%d", &pri)
		}
		results = append(results, WantedSummary{
			ID:          row["id"],
			Title:       row["title"],
			Project:     row["project"],
			Type:        row["type"],
			Priority:    pri,
			PostedBy:    row["posted_by"],
			ClaimedBy:   row["claimed_by"],
			Status:      row["status"],
			EffortLevel: row["effort_level"],
		})
	}
	return results
}
