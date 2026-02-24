package commons

import (
	"fmt"
	"strings"
)

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
}

// WantedSummary holds the columns returned by BrowseWanted.
type WantedSummary struct {
	ID          string
	Title       string
	Project     string
	Type        string
	Priority    int
	PostedBy    string
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
	if f.PostedBy != "" {
		conditions = append(conditions, fmt.Sprintf("posted_by = '%s'", EscapeSQL(f.PostedBy)))
	}
	if f.ClaimedBy != "" {
		conditions = append(conditions, fmt.Sprintf("claimed_by = '%s'", EscapeSQL(f.ClaimedBy)))
	}
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("title LIKE '%%%s%%'", EscapeSQL(f.Search)))
	}

	query := "SELECT id, title, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(posted_by,'') as posted_by, status, COALESCE(effort_level,'medium') as effort_level FROM wanted"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY priority ASC, created_at DESC"
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	return query
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
			Status:      row["status"],
			EffortLevel: row["effort_level"],
		})
	}
	return results
}
