package commons

import (
	"fmt"
	"strconv"
	"strings"
)

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
ORDER BY completions DESC, avg_quality DESC
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
		completions, _ := strconv.Atoi(row["completions"])
		avgQ, _ := strconv.ParseFloat(row["avg_quality"], 64)
		avgR, _ := strconv.ParseFloat(row["avg_reliability"], 64)

		entries = append(entries, LeaderboardEntry{
			RigHandle:   row["completed_by"],
			Completions: completions,
			AvgQuality:  avgQ,
			AvgReliab:   avgR,
		})
	}

	// Fetch top skills per rig.
	for i := range entries {
		entries[i].TopSkills = queryTopSkills(db, entries[i].RigHandle)
	}

	return entries, nil
}

// queryTopSkills returns the most frequent skill tags from stamps earned by a rig.
func queryTopSkills(db DB, rigHandle string) []string {
	// Stamps reference completions via context_id (context_type='completion').
	// We need skill_tags from stamps where the subject completed the work.
	query := fmt.Sprintf(`SELECT s.skill_tags
FROM stamps s
JOIN completions c ON s.context_id = c.id
WHERE c.completed_by = '%s' AND s.context_type = 'completion' AND s.skill_tags IS NOT NULL AND s.skill_tags != ''`,
		EscapeSQL(rigHandle))

	output, err := db.Query(query, "")
	if err != nil {
		return nil
	}

	rows := parseSimpleCSV(output)
	if len(rows) == 0 {
		return nil
	}

	// Count tag frequency across all stamps.
	freq := make(map[string]int)
	for _, row := range rows {
		tags := parseTagsJSON(row["skill_tags"])
		for _, tag := range tags {
			freq[strings.ToLower(tag)]++
		}
	}

	// Return top 5 by frequency.
	return topNKeys(freq, 5)
}

// topNKeys returns the top n keys from a frequency map, sorted by count descending.
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

	// Simple selection sort — n is small.
	for i := 0; i < len(sorted) && i < n; i++ {
		max := i
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[max].count {
				max = j
			}
		}
		sorted[i], sorted[max] = sorted[max], sorted[i]
	}

	result := make([]string, 0, n)
	for i := 0; i < len(sorted) && i < n; i++ {
		result = append(result, sorted[i].key)
	}
	return result
}
