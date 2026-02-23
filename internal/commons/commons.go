// Package commons provides wl-commons (Wasteland) database operations.
//
// The wl-commons database is the shared wanted board for the Wasteland federation.
// Phase 1 (wild-west mode): direct writes to main branch via the local Dolt CLI.
package commons

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// WLCommonsStore abstracts wl-commons database operations.
type WLCommonsStore interface {
	InsertWanted(item *WantedItem) error
	ClaimWanted(wantedID, rigHandle string) error
	SubmitCompletion(completionID, wantedID, rigHandle, evidence string) error
	QueryWanted(wantedID string) (*WantedItem, error)
	QueryCompletion(wantedID string) (*CompletionRecord, error)
	AcceptCompletion(wantedID, completionID, rigHandle string, stamp *Stamp) error
	UpdateWanted(wantedID string, fields *WantedUpdate) error
	DeleteWanted(wantedID string) error
}

// WLCommons implements WLCommonsStore using the real Dolt CLI.
type WLCommons struct{ dbDir string }

// NewWLCommons creates a WLCommonsStore backed by the real Dolt CLI.
// dbDir is the local fork clone directory (e.g., {dataDir}/{org}/{db}).
func NewWLCommons(dbDir string) *WLCommons { return &WLCommons{dbDir: dbDir} }

// InsertWanted inserts a new wanted item.
func (w *WLCommons) InsertWanted(item *WantedItem) error {
	return InsertWanted(w.dbDir, item)
}

// ClaimWanted claims a wanted item for a rig.
func (w *WLCommons) ClaimWanted(wantedID, rigHandle string) error {
	return ClaimWanted(w.dbDir, wantedID, rigHandle)
}

// SubmitCompletion records completion evidence for a claimed wanted item.
func (w *WLCommons) SubmitCompletion(completionID, wantedID, rigHandle, evidence string) error {
	return SubmitCompletion(w.dbDir, completionID, wantedID, rigHandle, evidence)
}

// QueryWanted fetches a wanted item by ID.
func (w *WLCommons) QueryWanted(wantedID string) (*WantedItem, error) {
	return QueryWanted(w.dbDir, wantedID)
}

// QueryCompletion fetches the completion record for a wanted item.
func (w *WLCommons) QueryCompletion(wantedID string) (*CompletionRecord, error) {
	return QueryCompletion(w.dbDir, wantedID)
}

// AcceptCompletion validates a completion and creates a stamp.
func (w *WLCommons) AcceptCompletion(wantedID, completionID, rigHandle string, stamp *Stamp) error {
	return AcceptCompletion(w.dbDir, wantedID, completionID, rigHandle, stamp)
}

// UpdateWanted updates mutable fields on an open wanted item.
func (w *WLCommons) UpdateWanted(wantedID string, fields *WantedUpdate) error {
	return UpdateWanted(w.dbDir, wantedID, fields)
}

// DeleteWanted soft-deletes a wanted item by setting status=withdrawn.
func (w *WLCommons) DeleteWanted(wantedID string) error {
	return DeleteWanted(w.dbDir, wantedID)
}

// WantedItem represents a row in the wanted table.
type WantedItem struct {
	ID              string
	Title           string
	Description     string
	Project         string
	Type            string
	Priority        int
	Tags            []string
	PostedBy        string
	ClaimedBy       string
	Status          string
	EffortLevel     string
	SandboxRequired bool
}

// CompletionRecord represents a row in the completions table.
type CompletionRecord struct {
	ID          string
	WantedID    string
	CompletedBy string
	Evidence    string
}

// Stamp represents a reputation stamp issued when accepting a completion.
type Stamp struct {
	ID          string
	Author      string
	Subject     string
	Quality     int
	Reliability int
	Severity    string
	ContextID   string
	ContextType string
	SkillTags   []string
	Message     string
}

// WantedUpdate holds the mutable fields for updating a wanted item.
// Zero-value fields are treated as "not set" and will not be updated.
// Priority uses -1 as "not set" since 0 is a valid priority.
type WantedUpdate struct {
	Title       string
	Description string
	Project     string
	Type        string
	Priority    int
	EffortLevel string
	Tags        []string
	TagsSet     bool // true if Tags was explicitly provided (even if empty)
}

// isNothingToCommit returns true if the error indicates DOLT_COMMIT found no
// changes to commit.
func isNothingToCommit(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "nothing to commit")
}

// EscapeSQL escapes backslashes and single quotes for SQL string literals.
func EscapeSQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "'", "''")
}

// GenerateWantedID generates a unique wanted item ID in the format w-<10-char-hash>.
func GenerateWantedID(title string) string {
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)

	input := fmt.Sprintf("%s:%d:%x", title, time.Now().UnixNano(), randomBytes)
	hash := sha256.Sum256([]byte(input))
	hashStr := hex.EncodeToString(hash[:])[:10]

	return fmt.Sprintf("w-%s", hashStr)
}

// GeneratePrefixedID generates a unique ID in the format <prefix>-<16 hex chars>
// from a SHA-256 hash of the inputs joined by "|" plus a timestamp.
func GeneratePrefixedID(prefix string, inputs ...string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	data := strings.Join(inputs, "|") + "|" + now
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%s-%x", prefix, h[:8])
}

// InsertWanted inserts a new wanted item into the wl-commons database.
// dbDir is the actual database directory.
func InsertWanted(dbDir string, item *WantedItem) error {
	if item.ID == "" {
		return fmt.Errorf("wanted item ID cannot be empty")
	}
	if item.Title == "" {
		return fmt.Errorf("wanted item title cannot be empty")
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	tagsJSON := formatTagsJSON(item.Tags)

	descField := "NULL"
	if item.Description != "" {
		descField = fmt.Sprintf("'%s'", EscapeSQL(item.Description))
	}
	projectField := "NULL"
	if item.Project != "" {
		projectField = fmt.Sprintf("'%s'", EscapeSQL(item.Project))
	}
	typeField := "NULL"
	if item.Type != "" {
		typeField = fmt.Sprintf("'%s'", EscapeSQL(item.Type))
	}
	postedByField := "NULL"
	if item.PostedBy != "" {
		postedByField = fmt.Sprintf("'%s'", EscapeSQL(item.PostedBy))
	}
	effortField := "'medium'"
	if item.EffortLevel != "" {
		effortField = fmt.Sprintf("'%s'", EscapeSQL(item.EffortLevel))
	}
	status := "'open'"
	if item.Status != "" {
		status = fmt.Sprintf("'%s'", EscapeSQL(item.Status))
	}

	script := fmt.Sprintf(`INSERT INTO wanted (id, title, description, project, type, priority, tags, posted_by, status, effort_level, created_at, updated_at)
VALUES ('%s', '%s', %s, %s, %s, %d, %s, %s, %s, %s, '%s', '%s');

CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'wl post: %s');
`,
		EscapeSQL(item.ID), EscapeSQL(item.Title), descField, projectField, typeField,
		item.Priority, tagsJSON, postedByField, status, effortField,
		now, now,
		EscapeSQL(item.Title))

	return doltSQLScript(dbDir, script)
}

// ClaimWanted updates a wanted item's status to claimed.
// dbDir is the actual database directory.
func ClaimWanted(dbDir, wantedID, rigHandle string) error {
	script := fmt.Sprintf(`UPDATE wanted SET claimed_by='%s', status='claimed', updated_at=NOW()
  WHERE id='%s' AND status='open';
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'wl claim: %s');
`, EscapeSQL(rigHandle), EscapeSQL(wantedID), EscapeSQL(wantedID))

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not open or does not exist", wantedID)
	}
	return fmt.Errorf("claim failed: %w", err)
}

// SubmitCompletion inserts a completion record and updates the wanted status.
// dbDir is the actual database directory.
func SubmitCompletion(dbDir, completionID, wantedID, rigHandle, evidence string) error {
	script := fmt.Sprintf(`UPDATE wanted SET status='in_review', evidence_url='%s', updated_at=NOW()
  WHERE id='%s' AND status='claimed' AND claimed_by='%s';
INSERT IGNORE INTO completions (id, wanted_id, completed_by, evidence, completed_at)
  SELECT '%s', '%s', '%s', '%s', NOW()
  FROM wanted WHERE id='%s' AND status='in_review' AND claimed_by='%s'
  AND NOT EXISTS (SELECT 1 FROM completions WHERE wanted_id='%s');
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'wl done: %s');
`,
		EscapeSQL(evidence), EscapeSQL(wantedID), EscapeSQL(rigHandle),
		EscapeSQL(completionID), EscapeSQL(wantedID), EscapeSQL(rigHandle), EscapeSQL(evidence),
		EscapeSQL(wantedID), EscapeSQL(rigHandle), EscapeSQL(wantedID),
		EscapeSQL(wantedID))

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not claimed by %q or does not exist", wantedID, rigHandle)
	}
	return fmt.Errorf("completion failed: %w", err)
}

// QueryWanted fetches a wanted item by ID. Returns an error if not found.
// dbDir is the actual database directory.
func QueryWanted(dbDir, wantedID string) (*WantedItem, error) {
	query := fmt.Sprintf(`SELECT id, title, status, COALESCE(claimed_by, '') as claimed_by FROM wanted WHERE id='%s';`,
		EscapeSQL(wantedID))

	output, err := doltSQLQuery(dbDir, query)
	if err != nil {
		return nil, err
	}

	rows := parseSimpleCSV(output)
	if len(rows) == 0 {
		return nil, fmt.Errorf("wanted item %q not found", wantedID)
	}

	row := rows[0]
	item := &WantedItem{
		ID:        row["id"],
		Title:     row["title"],
		Status:    row["status"],
		ClaimedBy: row["claimed_by"],
	}
	return item, nil
}

// parseSimpleCSV parses CSV output from dolt sql into a slice of maps.
func parseSimpleCSV(data string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(data), "\n")
	if len(lines) < 2 {
		return nil
	}

	headers := parseCSVLine(lines[0])
	var result []map[string]string

	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := parseCSVLine(line)
		row := make(map[string]string)
		for i, h := range headers {
			if i < len(fields) {
				row[strings.TrimSpace(h)] = strings.TrimSpace(fields[i])
			}
		}
		result = append(result, row)
	}
	return result
}

// parseCSVLine parses a single CSV line, handling quoted fields.
func parseCSVLine(line string) []string {
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

// QueryCompletion fetches the completion record for a wanted item.
// dbDir is the actual database directory.
func QueryCompletion(dbDir, wantedID string) (*CompletionRecord, error) {
	query := fmt.Sprintf(`SELECT id, wanted_id, completed_by, COALESCE(evidence, '') as evidence FROM completions WHERE wanted_id='%s';`,
		EscapeSQL(wantedID))

	output, err := doltSQLQuery(dbDir, query)
	if err != nil {
		return nil, err
	}

	rows := parseSimpleCSV(output)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no completion found for wanted item %q", wantedID)
	}

	row := rows[0]
	return &CompletionRecord{
		ID:          row["id"],
		WantedID:    row["wanted_id"],
		CompletedBy: row["completed_by"],
		Evidence:    row["evidence"],
	}, nil
}

// AcceptCompletion validates a completion, creates a stamp, and marks the item completed.
// dbDir is the actual database directory.
func AcceptCompletion(dbDir, wantedID, completionID, rigHandle string, stamp *Stamp) error {
	tagsField := formatTagsJSON(stamp.SkillTags)

	msgField := "NULL"
	if stamp.Message != "" {
		msgField = fmt.Sprintf("'%s'", EscapeSQL(stamp.Message))
	}

	valence := fmt.Sprintf(`{"quality": %d, "reliability": %d}`, stamp.Quality, stamp.Reliability)

	script := fmt.Sprintf(`INSERT INTO stamps (id, author, subject, valence, confidence, severity, context_id, context_type, skill_tags, message, created_at)
VALUES ('%s', '%s', '%s', '%s', 1.0, '%s', '%s', 'completion', %s, %s, NOW());
UPDATE completions SET validated_by='%s', stamp_id='%s', validated_at=NOW() WHERE id='%s';
UPDATE wanted SET status='completed', updated_at=NOW() WHERE id='%s' AND status='in_review';
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'wl accept: %s');
`,
		EscapeSQL(stamp.ID), EscapeSQL(rigHandle), EscapeSQL(stamp.Subject),
		EscapeSQL(valence), EscapeSQL(stamp.Severity),
		EscapeSQL(completionID), tagsField, msgField,
		EscapeSQL(rigHandle), EscapeSQL(stamp.ID), EscapeSQL(completionID),
		EscapeSQL(wantedID),
		EscapeSQL(wantedID))

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not in_review or does not exist", wantedID)
	}
	return fmt.Errorf("accept failed: %w", err)
}

// UpdateWanted updates mutable fields on an open wanted item.
// dbDir is the actual database directory.
func UpdateWanted(dbDir, wantedID string, fields *WantedUpdate) error {
	var setClauses []string

	if fields.Title != "" {
		setClauses = append(setClauses, fmt.Sprintf("title='%s'", EscapeSQL(fields.Title)))
	}
	if fields.Description != "" {
		setClauses = append(setClauses, fmt.Sprintf("description='%s'", EscapeSQL(fields.Description)))
	}
	if fields.Project != "" {
		setClauses = append(setClauses, fmt.Sprintf("project='%s'", EscapeSQL(fields.Project)))
	}
	if fields.Type != "" {
		setClauses = append(setClauses, fmt.Sprintf("type='%s'", EscapeSQL(fields.Type)))
	}
	if fields.Priority >= 0 {
		setClauses = append(setClauses, fmt.Sprintf("priority=%d", fields.Priority))
	}
	if fields.EffortLevel != "" {
		setClauses = append(setClauses, fmt.Sprintf("effort_level='%s'", EscapeSQL(fields.EffortLevel)))
	}
	if fields.TagsSet {
		setClauses = append(setClauses, fmt.Sprintf("tags=%s", formatTagsJSON(fields.Tags)))
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	setClauses = append(setClauses, "updated_at=NOW()")

	script := fmt.Sprintf("UPDATE wanted SET %s WHERE id='%s' AND status='open';\nCALL DOLT_ADD('-A');\nCALL DOLT_COMMIT('-m', 'wl update: %s');\n",
		strings.Join(setClauses, ", "), EscapeSQL(wantedID), EscapeSQL(wantedID))

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not open or does not exist", wantedID)
	}
	return fmt.Errorf("update failed: %w", err)
}

// formatTagsJSON formats a string slice as a JSON array SQL literal.
func formatTagsJSON(tags []string) string {
	if len(tags) == 0 {
		return "NULL"
	}
	escaped := make([]string, len(tags))
	for i, t := range tags {
		t = strings.ReplaceAll(t, `\`, `\\`)
		t = strings.ReplaceAll(t, `"`, `\"`)
		t = strings.ReplaceAll(t, "'", "''")
		escaped[i] = t
	}
	return fmt.Sprintf("'[\"%s\"]'", strings.Join(escaped, `","`))
}

// DeleteWanted soft-deletes a wanted item by setting status=withdrawn.
// dbDir is the actual database directory.
func DeleteWanted(dbDir, wantedID string) error {
	script := fmt.Sprintf(`UPDATE wanted SET status='withdrawn', updated_at=NOW() WHERE id='%s' AND status='open';
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'wl delete: %s');
`, EscapeSQL(wantedID), EscapeSQL(wantedID))

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not open or does not exist", wantedID)
	}
	return fmt.Errorf("delete failed: %w", err)
}
