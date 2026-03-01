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
	View      string    // "mine" (default), "all", or "upstream"
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
func BrowseWanted(db DB, f BrowseFilter) ([]WantedSummary, error) {
	query := BuildBrowseQuery(f)
	csvData, err := db.Query(query, "")
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

// BranchOverride maps a wanted ID to its state on a local mutation branch.
type BranchOverride struct {
	WantedID  string
	Branch    string
	Status    string
	ClaimedBy string
}

// DetectBranchOverrides lists wl/<rigHandle>/* branches and queries
// each item's status via AS OF. Returns overrides for items whose branch
// status differs from their main status.
func DetectBranchOverrides(db DB, rigHandle string) []BranchOverride {
	prefix := fmt.Sprintf("wl/%s/", rigHandle)
	branches, err := db.Branches(prefix)
	if err != nil || len(branches) == 0 {
		return nil
	}

	var overrides []BranchOverride
	for _, branch := range branches {
		wantedID := strings.TrimPrefix(branch, prefix)
		branchStatus, branchClaimedBy := queryItemBranchState(db, wantedID, branch)
		if branchStatus == "" {
			continue
		}
		mainStatus, _, _ := QueryItemStatus(db, wantedID, "")
		if branchStatus != mainStatus {
			overrides = append(overrides, BranchOverride{
				WantedID:  wantedID,
				Branch:    branch,
				Status:    branchStatus,
				ClaimedBy: branchClaimedBy,
			})
		}
	}
	return overrides
}

// queryItemBranchState returns (status, claimed_by) for a wanted item on a branch.
func queryItemBranchState(db DB, wantedID, branch string) (string, string) {
	query := fmt.Sprintf(
		"SELECT status, COALESCE(claimed_by,'') AS claimed_by FROM wanted WHERE id = '%s'",
		EscapeSQL(wantedID),
	)
	out, err := db.Query(query, branch)
	if err != nil {
		return "", ""
	}
	rows := parseSimpleCSV(out)
	if len(rows) == 0 {
		return "", ""
	}
	return rows[0]["status"], rows[0]["claimed_by"]
}

// DetectAllBranchOverrides scans all wl/* branches (all rigs) and returns
// overrides for items whose branch status differs from main, plus a count
// of how many branches touch each wanted ID.
func DetectAllBranchOverrides(db DB) ([]BranchOverride, map[string]int) {
	branches, err := db.Branches("wl/")
	if err != nil || len(branches) == 0 {
		return nil, nil
	}

	counts := make(map[string]int)
	var overrides []BranchOverride
	seen := make(map[string]bool) // track first override per wanted ID
	for _, branch := range branches {
		// Branch format: wl/{rigHandle}/{wantedID}
		rest := strings.TrimPrefix(branch, "wl/")
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			continue
		}
		wantedID := rest[slashIdx+1:]
		if wantedID == "" {
			continue
		}
		counts[wantedID]++

		if seen[wantedID] {
			continue // already have an override for this item
		}

		branchStatus, branchClaimedBy := queryItemBranchState(db, wantedID, branch)
		if branchStatus == "" {
			continue
		}
		mainStatus, _, _ := QueryItemStatus(db, wantedID, "")
		if branchStatus != mainStatus {
			seen[wantedID] = true
			overrides = append(overrides, BranchOverride{
				WantedID:  wantedID,
				Branch:    branch,
				Status:    branchStatus,
				ClaimedBy: branchClaimedBy,
			})
		}
	}
	return overrides, counts
}

// ApplyBranchOverrides adjusts browse results to reflect branch mutations.
func ApplyBranchOverrides(db DB, items []WantedSummary, overrides []BranchOverride, statusFilter string) []WantedSummary {
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
			if o.ClaimedBy != "" {
				item.ClaimedBy = o.ClaimedBy
			}
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
		// Try main first; fall back to branch (item may only exist there).
		item, err := QueryWantedDetail(db, o.WantedID)
		if err != nil {
			item, err = QueryWantedDetailAsOf(db, o.WantedID, o.Branch)
		}
		if err == nil {
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
func FindBranchForItem(db DB, rigHandle, wantedID string) string {
	branch := BranchName(rigHandle, wantedID)
	branches, err := db.Branches(branch)
	if err != nil || len(branches) == 0 {
		return ""
	}
	for _, b := range branches {
		if b == branch {
			return branch
		}
	}
	return ""
}

// ValidStatuses returns the browse filter status cycle.
func ValidStatuses() []string {
	return []string{"open", "claimed", "in_review", "completed", ""}
}

// ValidTypes returns the browse filter type cycle.
func ValidTypes() []string {
	return []string{"", "feature", "bug", "design", "rfc", "docs", "inference"}
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
	InReview  []WantedSummary // status=in_review, posted_by=me OR claimed_by=me
	Completed []WantedSummary // status=completed, claimed_by=me, limit 5
}

// QueryMyDashboard fetches personal dashboard data for the given handle.
func QueryMyDashboard(db DB, handle string) (*DashboardData, error) {
	escaped := EscapeSQL(handle)
	data := &DashboardData{}

	// Claimed items.
	claimedQ := fmt.Sprintf(
		"SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted WHERE status = 'claimed' AND claimed_by = '%s' ORDER BY priority ASC, created_at DESC LIMIT 50",
		escaped)
	csv, err := db.Query(claimedQ, "")
	if err != nil {
		return nil, fmt.Errorf("dashboard claimed: %w", err)
	}
	data.Claimed = parseWantedSummaries(csv)

	// In-review items (posted by me or claimed by me).
	reviewQ := fmt.Sprintf(
		"SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted WHERE status = 'in_review' AND (posted_by = '%s' OR claimed_by = '%s') ORDER BY priority ASC, created_at DESC LIMIT 50",
		escaped, escaped)
	csv, err = db.Query(reviewQ, "")
	if err != nil {
		return nil, fmt.Errorf("dashboard in_review: %w", err)
	}
	data.InReview = parseWantedSummaries(csv)

	// Recent completions.
	completedQ := fmt.Sprintf(
		"SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted WHERE status = 'completed' AND claimed_by = '%s' ORDER BY created_at DESC LIMIT 5",
		escaped)
	csv, err = db.Query(completedQ, "")
	if err != nil {
		return nil, fmt.Errorf("dashboard completed: %w", err)
	}
	data.Completed = parseWantedSummaries(csv)

	return data, nil
}

// QueryMyDashboardBranchAware wraps QueryMyDashboard with branch overlay in PR mode.
func QueryMyDashboardBranchAware(db DB, mode, rigHandle string) (*DashboardData, error) {
	data, err := QueryMyDashboard(db, rigHandle)
	if err != nil {
		return nil, err
	}
	if mode != "pr" {
		return data, nil
	}

	overrides := DetectBranchOverrides(db, rigHandle)
	if len(overrides) == 0 {
		return data, nil
	}

	// Apply overrides to each section with its status+person filter.
	data.Claimed = applyDashboardOverrides(db, data.Claimed, overrides, "claimed", "claimed_by", rigHandle)
	data.InReview = applyDashboardOverrides(db, data.InReview, overrides, "in_review", "either", rigHandle)
	data.Completed = applyDashboardOverrides(db, data.Completed, overrides, "completed", "claimed_by", rigHandle)

	return data, nil
}

// applyDashboardOverrides applies branch overrides to a dashboard section.
func applyDashboardOverrides(db DB, items []WantedSummary, overrides []BranchOverride, statusFilter, personField, personValue string) []WantedSummary {
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
			if o.ClaimedBy != "" {
				item.ClaimedBy = o.ClaimedBy
			}
			if item.Status != statusFilter {
				continue // override made it not match
			}
		}
		result = append(result, item)
	}

	// Add branch-only items that now match this section.
	for _, o := range overrides {
		if applied[o.WantedID] || o.Status != statusFilter {
			continue
		}
		item, err := QueryWantedDetail(db, o.WantedID)
		if err != nil {
			item, err = QueryWantedDetailAsOf(db, o.WantedID, o.Branch)
		}
		if err != nil {
			continue
		}
		// Check person filter.
		match := false
		switch personField {
		case "claimed_by":
			match = item.ClaimedBy == personValue || o.ClaimedBy == personValue
		case "posted_by":
			match = item.PostedBy == personValue
		case "either":
			match = item.PostedBy == personValue || item.ClaimedBy == personValue || o.ClaimedBy == personValue
		}
		if !match {
			continue
		}
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

	return result
}

// BrowseWantedBranchAware wraps BrowseWanted with branch overlay in PR mode.
// The view parameter controls which branches are considered:
//   - "upstream": no overlay, pure main data
//   - "mine" (default): only the current rig's branches
//   - "all": all rigs' branches
//
// Returns items, pending counts per wanted ID, and an error.
func BrowseWantedBranchAware(db DB, mode, rigHandle string, f BrowseFilter) ([]WantedSummary, map[string]int, error) {
	items, err := BrowseWanted(db, f)
	if err != nil {
		return nil, nil, err
	}
	pendingIDs := make(map[string]int)
	if mode != "pr" || f.View == "upstream" {
		return items, pendingIDs, nil
	}

	view := f.View
	if view == "" {
		view = "mine"
	}

	if view == "all" {
		overrides, counts := DetectAllBranchOverrides(db)
		for id, c := range counts {
			pendingIDs[id] = c
		}
		items = ApplyBranchOverrides(db, items, overrides, f.Status)
	} else {
		// "mine": existing behavior
		overrides := DetectBranchOverrides(db, rigHandle)
		for _, o := range overrides {
			pendingIDs[o.WantedID] = 1
		}
		items = ApplyBranchOverrides(db, items, overrides, f.Status)
	}
	return items, pendingIDs, nil
}

// QueryFullDetail fetches a wanted item with all related records.
func QueryFullDetail(db DB, wantedID string) (*WantedItem, *CompletionRecord, *Stamp, error) {
	return queryFullDetailRef(db, wantedID, "")
}

// QueryFullDetailAsOf fetches a wanted item with all related records from a specific ref.
func QueryFullDetailAsOf(db DB, wantedID, ref string) (*WantedItem, *CompletionRecord, *Stamp, error) {
	return queryFullDetailRef(db, wantedID, ref)
}

func queryFullDetailRef(db DB, wantedID, ref string) (*WantedItem, *CompletionRecord, *Stamp, error) {
	item, err := queryWantedDetailRef(db, wantedID, ref)
	if err != nil {
		return nil, nil, nil, err
	}

	var completion *CompletionRecord
	var stamp *Stamp

	if item.Status == "in_review" || item.Status == "completed" {
		if c, err := queryCompletionRef(db, wantedID, ref); err == nil {
			completion = c
			if c.StampID != "" {
				if s, err := queryStampRef(db, c.StampID, ref); err == nil {
					stamp = s
				}
			}
		}
	}

	return item, completion, stamp, nil
}

// ItemState captures the complete picture of an item across main and branch.
type ItemState struct {
	WantedID   string
	Main       *WantedItem       // item as on main (nil if not found)
	Branch     *WantedItem       // item as on mutation branch (nil if no branch)
	BranchName string            // "" if no branch exists
	Completion *CompletionRecord // from effective source
	Stamp      *Stamp            // from effective source
}

// Effective returns the branch item if present, otherwise the main item.
func (s *ItemState) Effective() *WantedItem {
	if s.Branch != nil {
		return s.Branch
	}
	return s.Main
}

// EffectiveStatus returns the status from the effective item, or "".
func (s *ItemState) EffectiveStatus() string {
	if e := s.Effective(); e != nil {
		return e.Status
	}
	return ""
}

// Delta returns the human-readable delta label, or "" if no delta.
func (s *ItemState) Delta() string {
	if s.Branch == nil {
		return ""
	}
	mainStatus := ""
	if s.Main != nil {
		mainStatus = s.Main.Status
	}
	return ComputeDelta(mainStatus, s.Branch.Status, true)
}

// ResolveItemState gives the complete picture of an item without checkout.
func ResolveItemState(db DB, rigHandle, wantedID string) (*ItemState, error) {
	state := &ItemState{WantedID: wantedID}

	// Main state.
	if item, err := QueryWantedDetail(db, wantedID); err == nil {
		state.Main = item
	}

	// Branch state (if exists).
	branch := FindBranchForItem(db, rigHandle, wantedID)
	if branch != "" {
		state.BranchName = branch
		if item, err := QueryWantedDetailAsOf(db, wantedID, branch); err == nil {
			state.Branch = item
		}
	}

	// Completion + stamp from effective source.
	effective := state.Effective()
	if effective != nil && (effective.Status == "in_review" || effective.Status == "completed") {
		ref := ""
		if branch != "" {
			ref = branch
		}
		if c, err := queryCompletionRef(db, wantedID, ref); err == nil {
			state.Completion = c
			if c.StampID != "" {
				if s, err := queryStampRef(db, c.StampID, ref); err == nil {
					state.Stamp = s
				}
			}
		}
	}

	return state, nil
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
