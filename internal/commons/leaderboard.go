package commons

import (
	"fmt"
	"strconv"
	"strings"
)

// maxLeaderboardLimit is the hard ceiling for leaderboard queries to prevent
// unbounded data aggregation and excessive memory usage.
const maxLeaderboardLimit = 100

// LeaderboardEntry holds aggregated stats for one rig on the leaderboard.
type LeaderboardEntry struct {
	RigHandle   string
	Completions int
	AvgQuality  float64
	AvgReliab   float64
	TopSkills   []string // up to 5 most frequent skill tags
}

// QueryLeaderboard aggregates completions and stamps into a ranked leaderboard.
// Rigs are ranked by number of validated completions (those with a stamp_id).
func QueryLeaderboard(db DB, limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > maxLeaderboardLimit {
		limit = maxLeaderboardLimit
	}

	// Join completions with stamps to get per-rig aggregates.
	// Only count completions that have been validated (stamp_id IS NOT NULL).
	query := fmt.Sprintf(`SELECT
  c.completed_by,
  COUNT(*) AS completions,
  COALESCE(AVG(JSON_EXTRACT(s.valence, '$.quality')), 0) AS avg_quality,
  COALESCE(AVG(JSON_EXTRACT(s.valence, '$.reliability')), 0) AS avg_reliability
FROM completions c
JOIN stamps s ON c.stamp_id = s.id
GROUP BY c.completed_by
ORDER BY completions DESC, avg_quality DESC, c.completed_by ASC
LIMIT %d`, limit)

	output, err := db.Query(query, "")
	if err != nil {
		return nil, fmt.Errorf("querying leaderboard: %w", err)
	}

	rows := parseSimpleCSV(output)
	if len(rows) == 0 {
		return nil, nil
	}

	entries := make([]LeaderboardEntry, 0, len(rows))
	for _, row := range rows {
		completions, err := strconv.Atoi(row["completions"])
		if err != nil {
			return nil, fmt.Errorf("parsing completions for %q: %w", row["completed_by"], err)
		}
		avgQ, err := strconv.ParseFloat(row["avg_quality"], 64)
		if err != nil {
			return nil, fmt.Errorf("parsing avg_quality for %q: %w", row["completed_by"], err)
		}
		avgR, err := strconv.ParseFloat(row["avg_reliability"], 64)
		if err != nil {
			return nil, fmt.Errorf("parsing avg_reliability for %q: %w", row["completed_by"], err)
		}

		entries = append(entries, LeaderboardEntry{
			RigHandle:   row["completed_by"],
			Completions: completions,
			AvgQuality:  avgQ,
			AvgReliab:   avgR,
		})
	}

	// Fetch top skills for all rigs in a single query to avoid N+1.
	if err := populateTopSkills(db, entries); err != nil {
		return nil, fmt.Errorf("querying top skills: %w", err)
	}

	return entries, nil
}

// populateTopSkills fetches skill tags for all rigs in a single query and
// assigns the top 5 most frequent tags to each entry.
func populateTopSkills(db DB, entries []LeaderboardEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Build IN clause from rig handles.
	handles := make([]string, len(entries))
	for i, e := range entries {
		handles[i] = fmt.Sprintf("'%s'", EscapeSQL(e.RigHandle))
	}

	// Use the same join path as the main query (completions.stamp_id → stamps.id)
	// to ensure consistency between which stamps count for ranking vs skills.
	// No global LIMIT — the IN clause is bounded by the main query's limit (≤100 rigs).
	query := fmt.Sprintf(`SELECT c.completed_by, s.skill_tags
FROM completions c
JOIN stamps s ON c.stamp_id = s.id
WHERE c.completed_by IN (%s) AND s.skill_tags IS NOT NULL AND s.skill_tags != ''
ORDER BY c.completed_by`,
		strings.Join(handles, ","))

	output, err := db.Query(query, "")
	if err != nil {
		return err
	}

	rows := parseSimpleCSV(output)

	// Count tag frequency per rig. Use parseTagsJSON which skips malformed
	// entries — a single bad row should not break the entire leaderboard.
	perRig := make(map[string]map[string]int)
	for _, row := range rows {
		rig := row["completed_by"]
		tags := parseTagsJSON(row["skill_tags"])
		if len(tags) == 0 {
			continue
		}
		if perRig[rig] == nil {
			perRig[rig] = make(map[string]int)
		}
		for _, tag := range tags {
			perRig[rig][strings.ToLower(tag)]++
		}
	}

	for i := range entries {
		entries[i].TopSkills = topNKeys(perRig[entries[i].RigHandle], 5)
	}
	return nil
}

// topNKeys returns the top n keys from a frequency map, sorted by count
// descending then key ascending for deterministic output.
func topNKeys(freq map[string]int, n int) []string {
	if len(freq) == 0 {
		return nil
	}

	type kv struct {
		key   string
		count int
	}
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}

	// Simple selection sort — n is small. Break ties alphabetically.
	for i := 0; i < len(sorted) && i < n; i++ {
		best := i
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[best].count ||
				(sorted[j].count == sorted[best].count && sorted[j].key < sorted[best].key) {
				best = j
			}
		}
		sorted[i], sorted[best] = sorted[best], sorted[i]
	}

	result := make([]string, 0, n)
	for i := 0; i < len(sorted) && i < n; i++ {
		result = append(result, sorted[i].key)
	}
	return result
}
