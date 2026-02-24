// Package commons provides wl-commons (Wasteland) database operations.
//
// The wl-commons database is the shared wanted board for the Wasteland federation.
// Phase 1 (wild-west mode): direct writes to main branch via the local Dolt CLI.
package commons

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// WLCommonsStore abstracts wl-commons database operations.
type WLCommonsStore interface {
	InsertWanted(item *WantedItem) error
	ClaimWanted(wantedID, rigHandle string) error
	UnclaimWanted(wantedID string) error
	SubmitCompletion(completionID, wantedID, rigHandle, evidence string) error
	QueryWanted(wantedID string) (*WantedItem, error)
	QueryWantedDetail(wantedID string) (*WantedItem, error)
	QueryCompletion(wantedID string) (*CompletionRecord, error)
	QueryStamp(stampID string) (*Stamp, error)
	AcceptCompletion(wantedID, completionID, rigHandle string, stamp *Stamp) error
	RejectCompletion(wantedID, rigHandle, reason string) error
	CloseWanted(wantedID string) error
	UpdateWanted(wantedID string, fields *WantedUpdate) error
	DeleteWanted(wantedID string) error
}

// WLCommons implements WLCommonsStore using the real Dolt CLI.
type WLCommons struct {
	dbDir  string
	signed bool
	hopURI string
}

// NewWLCommons creates a WLCommonsStore backed by the real Dolt CLI.
// dbDir is the local fork clone directory (e.g., {dataDir}/{org}/{db}).
func NewWLCommons(dbDir string) *WLCommons { return &WLCommons{dbDir: dbDir} }

// SetSigning enables or disables GPG-signed Dolt commits.
func (w *WLCommons) SetSigning(enabled bool) { w.signed = enabled }

// SetHopURI sets the rig's HOP protocol URI for completions and stamps.
func (w *WLCommons) SetHopURI(uri string) { w.hopURI = uri }

// InsertWanted inserts a new wanted item.
func (w *WLCommons) InsertWanted(item *WantedItem) error {
	return InsertWanted(w.dbDir, item, w.signed)
}

// ClaimWanted claims a wanted item for a rig.
func (w *WLCommons) ClaimWanted(wantedID, rigHandle string) error {
	return ClaimWanted(w.dbDir, wantedID, rigHandle, w.signed)
}

// UnclaimWanted reverts a claimed wanted item to open.
func (w *WLCommons) UnclaimWanted(wantedID string) error {
	return UnclaimWanted(w.dbDir, wantedID, w.signed)
}

// SubmitCompletion records completion evidence for a claimed wanted item.
func (w *WLCommons) SubmitCompletion(completionID, wantedID, rigHandle, evidence string) error {
	return SubmitCompletion(w.dbDir, completionID, wantedID, rigHandle, evidence, w.hopURI, w.signed)
}

// QueryWanted fetches a wanted item by ID.
func (w *WLCommons) QueryWanted(wantedID string) (*WantedItem, error) {
	return QueryWanted(w.dbDir, wantedID)
}

// QueryWantedDetail fetches a wanted item with all display fields.
func (w *WLCommons) QueryWantedDetail(wantedID string) (*WantedItem, error) {
	return QueryWantedDetail(w.dbDir, wantedID)
}

// QueryCompletion fetches the completion record for a wanted item.
func (w *WLCommons) QueryCompletion(wantedID string) (*CompletionRecord, error) {
	return QueryCompletion(w.dbDir, wantedID)
}

// QueryStamp fetches a stamp by ID.
func (w *WLCommons) QueryStamp(stampID string) (*Stamp, error) {
	return QueryStamp(w.dbDir, stampID)
}

// AcceptCompletion validates a completion and creates a stamp.
func (w *WLCommons) AcceptCompletion(wantedID, completionID, rigHandle string, stamp *Stamp) error {
	return AcceptCompletion(w.dbDir, wantedID, completionID, rigHandle, w.hopURI, stamp, w.signed)
}

// UpdateWanted updates mutable fields on an open wanted item.
func (w *WLCommons) UpdateWanted(wantedID string, fields *WantedUpdate) error {
	return UpdateWanted(w.dbDir, wantedID, fields, w.signed)
}

// RejectCompletion reverts a wanted item from in_review to claimed.
func (w *WLCommons) RejectCompletion(wantedID, rigHandle, reason string) error {
	return RejectCompletion(w.dbDir, wantedID, rigHandle, reason, w.signed)
}

// CloseWanted marks an in_review item as completed without a stamp.
func (w *WLCommons) CloseWanted(wantedID string) error {
	return CloseWanted(w.dbDir, wantedID, w.signed)
}

// DeleteWanted soft-deletes a wanted item by setting status=withdrawn.
func (w *WLCommons) DeleteWanted(wantedID string) error {
	return DeleteWanted(w.dbDir, wantedID, w.signed)
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
	CreatedAt       string
	UpdatedAt       string
}

// CompletionRecord represents a row in the completions table.
type CompletionRecord struct {
	ID          string
	WantedID    string
	CompletedBy string
	Evidence    string
	StampID     string
	ValidatedBy string
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

// commitSQL returns the DOLT_COMMIT SQL statement, optionally with -S for GPG signing.
func commitSQL(msg string, signed bool) string {
	if signed {
		return fmt.Sprintf("CALL DOLT_COMMIT('-S', '-m', '%s');\n", EscapeSQL(msg))
	}
	return fmt.Sprintf("CALL DOLT_COMMIT('-m', '%s');\n", EscapeSQL(msg))
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
func InsertWanted(dbDir string, item *WantedItem, signed bool) error {
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
`,
		EscapeSQL(item.ID), EscapeSQL(item.Title), descField, projectField, typeField,
		item.Priority, tagsJSON, postedByField, status, effortField,
		now, now)
	script += commitSQL("wl post: "+item.Title, signed)

	return doltSQLScript(dbDir, script)
}

// ClaimWanted updates a wanted item's status to claimed.
// dbDir is the actual database directory.
func ClaimWanted(dbDir, wantedID, rigHandle string, signed bool) error {
	script := fmt.Sprintf("UPDATE wanted SET claimed_by='%s', status='claimed', updated_at=NOW()\n  WHERE id='%s' AND status='open';\nCALL DOLT_ADD('-A');\n",
		EscapeSQL(rigHandle), EscapeSQL(wantedID))
	script += commitSQL("wl claim: "+wantedID, signed)

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not open or does not exist", wantedID)
	}
	return fmt.Errorf("claim failed: %w", err)
}

// UnclaimWanted reverts a claimed wanted item to open, clearing claimed_by.
// dbDir is the actual database directory.
func UnclaimWanted(dbDir, wantedID string, signed bool) error {
	script := fmt.Sprintf("UPDATE wanted SET claimed_by=NULL, status='open', updated_at=NOW()\n  WHERE id='%s' AND status='claimed';\nCALL DOLT_ADD('-A');\n",
		EscapeSQL(wantedID))
	script += commitSQL("wl unclaim: "+wantedID, signed)

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not claimed or does not exist", wantedID)
	}
	return fmt.Errorf("unclaim failed: %w", err)
}

// SubmitCompletion inserts a completion record and updates the wanted status.
// dbDir is the actual database directory.
func SubmitCompletion(dbDir, completionID, wantedID, rigHandle, evidence, hopURI string, signed bool) error {
	hopField := "NULL"
	if hopURI != "" {
		hopField = fmt.Sprintf("'%s'", EscapeSQL(hopURI))
	}

	script := fmt.Sprintf(`UPDATE wanted SET status='in_review', evidence_url='%s', updated_at=NOW()
  WHERE id='%s' AND status='claimed' AND claimed_by='%s';
INSERT IGNORE INTO completions (id, wanted_id, completed_by, evidence, hop_uri, completed_at)
  SELECT '%s', '%s', '%s', '%s', %s, NOW()
  FROM wanted WHERE id='%s' AND status='in_review' AND claimed_by='%s'
  AND NOT EXISTS (SELECT 1 FROM completions WHERE wanted_id='%s');
CALL DOLT_ADD('-A');
`,
		EscapeSQL(evidence), EscapeSQL(wantedID), EscapeSQL(rigHandle),
		EscapeSQL(completionID), EscapeSQL(wantedID), EscapeSQL(rigHandle), EscapeSQL(evidence),
		hopField,
		EscapeSQL(wantedID), EscapeSQL(rigHandle), EscapeSQL(wantedID))
	script += commitSQL("wl done: "+wantedID, signed)

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
	query := fmt.Sprintf(`SELECT id, title, status, COALESCE(claimed_by, '') as claimed_by, COALESCE(posted_by, '') as posted_by FROM wanted WHERE id='%s';`,
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
		PostedBy:  row["posted_by"],
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
	query := fmt.Sprintf(`SELECT id, wanted_id, completed_by, COALESCE(evidence, '') as evidence, COALESCE(stamp_id, '') as stamp_id, COALESCE(validated_by, '') as validated_by FROM completions WHERE wanted_id='%s';`,
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
		StampID:     row["stamp_id"],
		ValidatedBy: row["validated_by"],
	}, nil
}

// QueryWantedDetail fetches a wanted item with all display fields.
// dbDir is the actual database directory.
func QueryWantedDetail(dbDir, wantedID string) (*WantedItem, error) {
	query := fmt.Sprintf(`SELECT id, title, COALESCE(description,'') as description, COALESCE(project,'') as project, COALESCE(type,'') as type, priority, COALESCE(tags,'') as tags, COALESCE(posted_by,'') as posted_by, COALESCE(claimed_by,'') as claimed_by, status, COALESCE(effort_level,'medium') as effort_level, COALESCE(created_at,'') as created_at, COALESCE(updated_at,'') as updated_at FROM wanted WHERE id='%s';`,
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
	priority, _ := strconv.Atoi(row["priority"])

	return &WantedItem{
		ID:          row["id"],
		Title:       row["title"],
		Description: row["description"],
		Project:     row["project"],
		Type:        row["type"],
		Priority:    priority,
		Tags:        parseTagsJSON(row["tags"]),
		PostedBy:    row["posted_by"],
		ClaimedBy:   row["claimed_by"],
		Status:      row["status"],
		EffortLevel: row["effort_level"],
		CreatedAt:   row["created_at"],
		UpdatedAt:   row["updated_at"],
	}, nil
}

// QueryStamp fetches a stamp by ID.
// dbDir is the actual database directory.
func QueryStamp(dbDir, stampID string) (*Stamp, error) {
	query := fmt.Sprintf(`SELECT id, author, subject, valence, severity, COALESCE(context_id,'') as context_id, COALESCE(context_type,'') as context_type, COALESCE(skill_tags,'') as skill_tags, COALESCE(message,'') as message FROM stamps WHERE id='%s';`,
		EscapeSQL(stampID))

	output, err := doltSQLQuery(dbDir, query)
	if err != nil {
		return nil, err
	}

	rows := parseSimpleCSV(output)
	if len(rows) == 0 {
		return nil, fmt.Errorf("stamp %q not found", stampID)
	}

	row := rows[0]

	var valence struct {
		Quality     int `json:"quality"`
		Reliability int `json:"reliability"`
	}
	if v := row["valence"]; v != "" {
		_ = json.Unmarshal([]byte(v), &valence)
	}

	return &Stamp{
		ID:          row["id"],
		Author:      row["author"],
		Subject:     row["subject"],
		Quality:     valence.Quality,
		Reliability: valence.Reliability,
		Severity:    row["severity"],
		ContextID:   row["context_id"],
		ContextType: row["context_type"],
		SkillTags:   parseTagsJSON(row["skill_tags"]),
		Message:     row["message"],
	}, nil
}

// parseTagsJSON parses a JSON array string like `["go","auth"]` into a string slice.
func parseTagsJSON(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "NULL" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return nil
	}
	return tags
}

// AcceptCompletion validates a completion, creates a stamp, and marks the item completed.
// dbDir is the actual database directory.
func AcceptCompletion(dbDir, wantedID, completionID, rigHandle, hopURI string, stamp *Stamp, signed bool) error {
	tagsField := formatTagsJSON(stamp.SkillTags)

	msgField := "NULL"
	if stamp.Message != "" {
		msgField = fmt.Sprintf("'%s'", EscapeSQL(stamp.Message))
	}

	hopField := "NULL"
	if hopURI != "" {
		hopField = fmt.Sprintf("'%s'", EscapeSQL(hopURI))
	}

	valence := fmt.Sprintf(`{"quality": %d, "reliability": %d}`, stamp.Quality, stamp.Reliability)

	script := fmt.Sprintf(`INSERT INTO stamps (id, author, subject, valence, confidence, severity, context_id, context_type, skill_tags, message, hop_uri, created_at)
VALUES ('%s', '%s', '%s', '%s', 1.0, '%s', '%s', 'completion', %s, %s, %s, NOW());
UPDATE completions SET validated_by='%s', stamp_id='%s', validated_at=NOW() WHERE id='%s';
UPDATE wanted SET status='completed', updated_at=NOW() WHERE id='%s' AND status='in_review';
CALL DOLT_ADD('-A');
`,
		EscapeSQL(stamp.ID), EscapeSQL(rigHandle), EscapeSQL(stamp.Subject),
		EscapeSQL(valence), EscapeSQL(stamp.Severity),
		EscapeSQL(completionID), tagsField, msgField, hopField,
		EscapeSQL(rigHandle), EscapeSQL(stamp.ID), EscapeSQL(completionID),
		EscapeSQL(wantedID))
	script += commitSQL("wl accept: "+wantedID, signed)

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
func UpdateWanted(dbDir, wantedID string, fields *WantedUpdate, signed bool) error {
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

	script := fmt.Sprintf("UPDATE wanted SET %s WHERE id='%s' AND status='open';\nCALL DOLT_ADD('-A');\n",
		strings.Join(setClauses, ", "), EscapeSQL(wantedID))
	script += commitSQL("wl update: "+wantedID, signed)

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not open or does not exist", wantedID)
	}
	return fmt.Errorf("update failed: %w", err)
}

// CloseWanted marks an in_review wanted item as completed without issuing a
// stamp. This is housekeeping for solo maintainers who completed their own work.
// dbDir is the actual database directory.
func CloseWanted(dbDir, wantedID string, signed bool) error {
	script := fmt.Sprintf("UPDATE wanted SET status='completed', updated_at=NOW() WHERE id='%s' AND status='in_review';\nCALL DOLT_ADD('-A');\n",
		EscapeSQL(wantedID))
	script += commitSQL("wl close: "+wantedID, signed)

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not in_review or does not exist", wantedID)
	}
	return fmt.Errorf("close failed: %w", err)
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
		escaped[i] = t
	}
	jsonStr := fmt.Sprintf(`["%s"]`, strings.Join(escaped, `","`))
	return fmt.Sprintf("'%s'", strings.ReplaceAll(jsonStr, "'", "''"))
}

// DeleteWanted soft-deletes a wanted item by setting status=withdrawn.
// dbDir is the actual database directory.
func DeleteWanted(dbDir, wantedID string, signed bool) error {
	script := fmt.Sprintf("UPDATE wanted SET status='withdrawn', updated_at=NOW() WHERE id='%s' AND status='open';\nCALL DOLT_ADD('-A');\n",
		EscapeSQL(wantedID))
	script += commitSQL("wl delete: "+wantedID, signed)

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not open or does not exist", wantedID)
	}
	return fmt.Errorf("delete failed: %w", err)
}

// RejectCompletion reverts a wanted item from in_review to claimed and deletes
// the completion record. The reason is embedded in the dolt commit message.
// dbDir is the actual database directory.
func RejectCompletion(dbDir, wantedID, _, reason string, signed bool) error {
	commitMsg := fmt.Sprintf("wl reject: %s", wantedID)
	if reason != "" {
		commitMsg += " — " + reason
	}

	script := fmt.Sprintf("DELETE FROM completions WHERE wanted_id='%s';\nUPDATE wanted SET status='claimed', updated_at=NOW() WHERE id='%s' AND status='in_review';\nCALL DOLT_ADD('-A');\n",
		EscapeSQL(wantedID), EscapeSQL(wantedID))
	script += commitSQL(commitMsg, signed)

	err := doltSQLScript(dbDir, script)
	if err == nil {
		return nil
	}
	if isNothingToCommit(err) {
		return fmt.Errorf("wanted item %q is not in_review or does not exist", wantedID)
	}
	return fmt.Errorf("reject failed: %w", err)
}
