package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DoltHubAPIBase is the DoltHub REST API base URL. Var so tests can override.
var DoltHubAPIBase = "https://www.dolthub.com/api/v1alpha1"

// RemoteDB implements DB using the DoltHub REST API.
// Reads from main go to the upstream (shared) database.
// Branch reads and all writes go to the fork (user's) database.
type RemoteDB struct {
	token      string
	readOwner  string // upstream org
	readDB     string // upstream db name
	writeOwner string // fork org
	writeDB    string // fork db name
	mode       string // "pr" or "wild-west"
	client     *http.Client
}

// NewRemoteDB creates a DB backed by the DoltHub REST API.
func NewRemoteDB(token, readOwner, readDB, writeOwner, writeDB, mode string) *RemoteDB {
	return &RemoteDB{
		token:      token,
		readOwner:  readOwner,
		readDB:     readDB,
		writeOwner: writeOwner,
		writeDB:    writeDB,
		mode:       mode,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Query runs a read-only SQL SELECT via the DoltHub API.
func (r *RemoteDB) Query(sql, ref string) (string, error) {
	owner := r.readOwner
	db := r.readDB
	branch := "main"

	if ref != "" {
		// Branch refs read from the fork database.
		owner = r.writeOwner
		db = r.writeDB
		branch = ref
	}

	apiURL := fmt.Sprintf("%s/%s/%s/%s?q=%s",
		DoltHubAPIBase, owner, db, url.PathEscape(branch), url.QueryEscape(sql))

	body, err := r.doGet(apiURL)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	return JSONToCSV(body)
}

// Exec runs DML via the DoltHub write API on the given branch.
func (r *RemoteDB) Exec(branch, _ string, _ bool, stmts ...string) error {
	if branch == "" {
		branch = "main"
	}

	// Join all statements with semicolons.
	joined := strings.Join(stmts, ";\n") + ";"

	apiURL := fmt.Sprintf("%s/%s/%s/write/main/%s?q=%s",
		DoltHubAPIBase, r.writeOwner, r.writeDB, branch,
		url.QueryEscape(joined))

	body, err := r.doPost(apiURL, nil)
	if err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	// The write API returns an operation name to poll.
	var writeResp struct {
		OperationName         string `json:"operation_name"`
		QueryExecutionStatus  string `json:"query_execution_status"`
		QueryExecutionMessage string `json:"query_execution_message"`
	}
	if err := json.Unmarshal(body, &writeResp); err != nil {
		return fmt.Errorf("parsing write response: %w", err)
	}

	if writeResp.QueryExecutionStatus == "Error" {
		return fmt.Errorf("exec error: %s", writeResp.QueryExecutionMessage)
	}

	// If there's an operation to poll, wait for it.
	if writeResp.OperationName != "" {
		return r.pollOperation(writeResp.OperationName)
	}

	// Some writes complete synchronously.
	return nil
}

// Branches returns branch names matching the given prefix from the fork.
func (r *RemoteDB) Branches(prefix string) ([]string, error) {
	sql := fmt.Sprintf("SELECT name FROM dolt_branches WHERE name LIKE '%s%%' ORDER BY name",
		strings.ReplaceAll(prefix, "'", "''"))

	// Query branches on the fork database.
	apiURL := fmt.Sprintf("%s/%s/%s/main?q=%s",
		DoltHubAPIBase, r.writeOwner, r.writeDB, url.QueryEscape(sql))

	body, err := r.doGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("branches query failed: %w", err)
	}

	csv, err := JSONToCSV(body)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) < 2 {
		return nil, nil
	}
	var branches []string
	for _, line := range lines[1:] {
		name := strings.TrimSpace(line)
		if name != "" {
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// DeleteBranch removes a branch on the fork via the write API.
func (r *RemoteDB) DeleteBranch(name string) error {
	escaped := strings.ReplaceAll(name, "'", "''")
	sql := fmt.Sprintf("CALL DOLT_BRANCH('-D', '%s')", escaped)

	apiURL := fmt.Sprintf("%s/%s/%s/write/main/main?q=%s",
		DoltHubAPIBase, r.writeOwner, r.writeDB, url.QueryEscape(sql))

	body, err := r.doPost(apiURL, nil)
	if err != nil {
		return fmt.Errorf("delete branch failed: %w", err)
	}

	var writeResp struct {
		OperationName string `json:"operation_name"`
	}
	if err := json.Unmarshal(body, &writeResp); err == nil && writeResp.OperationName != "" {
		return r.pollOperation(writeResp.OperationName)
	}

	return nil
}

// PushBranch is a no-op for remote — the write API auto-pushes.
func (r *RemoteDB) PushBranch(_ string, _ io.Writer) error { return nil }

// PushMain is a no-op for remote.
func (r *RemoteDB) PushMain(_ io.Writer) error { return nil }

// PushWithSync is a no-op for remote.
func (r *RemoteDB) PushWithSync(_ io.Writer) error { return nil }

// Sync attempts to sync the fork's main branch from upstream.
// It ensures an upstream remote exists on the fork, fetches it, and then
// resets (PR mode) or merges (wild-west mode) fork main to upstream/main.
//
// Best-effort: DoltHub's hosted SQL API may not support remote operations
// (dolt_remotes, DOLT_REMOTE, DOLT_FETCH). Callers should treat errors as
// non-fatal — reads always go to the upstream API and are always fresh.
func (r *RemoteDB) Sync() error {
	if err := r.ensureUpstreamRemote(); err != nil {
		return fmt.Errorf("remote operations not supported: %w", err)
	}
	if err := r.execOnMain("CALL DOLT_FETCH('upstream')"); err != nil {
		return fmt.Errorf("fetch upstream: %w", err)
	}
	if r.mode == "pr" {
		if err := r.execOnMain("CALL DOLT_RESET('--hard', 'upstream/main')"); err != nil {
			return fmt.Errorf("reset to upstream: %w", err)
		}
	} else {
		if err := r.execOnMain("CALL DOLT_MERGE('upstream/main')"); err != nil {
			return fmt.Errorf("merge upstream: %w", err)
		}
	}
	return nil
}

// MergeBranch merges a branch into the fork's main via the write API.
func (r *RemoteDB) MergeBranch(branch string) error {
	escaped := strings.ReplaceAll(branch, "'", "''")
	return r.execOnMain(fmt.Sprintf("CALL DOLT_MERGE('%s')", escaped))
}

// DeleteRemoteBranch removes a branch on the fork. For remote backend, local
// and remote branches are the same thing (both live on the fork).
func (r *RemoteDB) DeleteRemoteBranch(branch string) error {
	return r.DeleteBranch(branch)
}

// Diff returns a human-readable diff of changes on the given branch
// relative to the fork's main, querying dolt system diff tables via the API.
func (r *RemoteDB) Diff(branch string) (string, error) {
	escaped := strings.ReplaceAll(branch, "'", "''")

	// List changed tables via dolt_diff_stat (2-arg form: from, to).
	tableSQL := fmt.Sprintf(
		"SELECT table_name, rows_added, rows_modified, rows_deleted FROM dolt_diff_stat('main', '%s')", escaped)
	tableCSV, err := r.queryForkBranch(tableSQL, branch)
	if err != nil {
		return "", fmt.Errorf("diff: listing changed tables: %w", err)
	}

	tables := parseDiffTables(tableCSV)
	if len(tables) == 0 {
		return "(no changes)\n", nil
	}

	var buf strings.Builder
	for _, tbl := range tables {
		fmt.Fprintf(&buf, "## %s\n\n", tbl)

		// Query row-level changes via dolt_diff (3-arg form: from, to, table).
		rowSQL := fmt.Sprintf(
			"SELECT * FROM dolt_diff('main', '%s', '%s')",
			escaped, strings.ReplaceAll(tbl, "'", "''"))
		rowCSV, err := r.queryForkBranch(rowSQL, branch)
		if err != nil {
			fmt.Fprintf(&buf, "(error reading diff: %v)\n\n", err)
			continue
		}

		lines := strings.Split(strings.TrimSpace(rowCSV), "\n")
		if len(lines) < 2 {
			fmt.Fprintf(&buf, "(no row changes)\n\n")
			continue
		}

		header := strings.Split(lines[0], ",")
		buf.WriteString("```\n")
		for _, row := range lines[1:] {
			fields := strings.Split(row, ",")
			formatDiffRow(&buf, header, fields)
		}
		buf.WriteString("```\n\n")
	}

	return buf.String(), nil
}

// --- Remote helpers ---

// execOnMain posts a SQL statement to the write API on the fork's main branch
// and polls until the operation completes.
func (r *RemoteDB) execOnMain(sql string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/write/main/main?q=%s",
		DoltHubAPIBase, r.writeOwner, r.writeDB, url.QueryEscape(sql))

	body, err := r.doPost(apiURL, nil)
	if err != nil {
		return fmt.Errorf("execOnMain failed: %w", err)
	}

	var writeResp struct {
		OperationName         string `json:"operation_name"`
		QueryExecutionStatus  string `json:"query_execution_status"`
		QueryExecutionMessage string `json:"query_execution_message"`
	}
	if err := json.Unmarshal(body, &writeResp); err != nil {
		return fmt.Errorf("parsing write response: %w", err)
	}

	if writeResp.QueryExecutionStatus == "Error" {
		return fmt.Errorf("exec error: %s", writeResp.QueryExecutionMessage)
	}

	if writeResp.OperationName != "" {
		return r.pollOperation(writeResp.OperationName)
	}

	return nil
}

// queryForkMain runs a read-only SELECT against the fork's main branch.
func (r *RemoteDB) queryForkMain(sql string) (string, error) {
	apiURL := fmt.Sprintf("%s/%s/%s/main?q=%s",
		DoltHubAPIBase, r.writeOwner, r.writeDB, url.QueryEscape(sql))

	body, err := r.doGet(apiURL)
	if err != nil {
		return "", fmt.Errorf("queryForkMain failed: %w", err)
	}

	return JSONToCSV(body)
}

// queryForkBranch runs a read-only SELECT against a specific branch on the fork.
func (r *RemoteDB) queryForkBranch(sql, branch string) (string, error) {
	apiURL := fmt.Sprintf("%s/%s/%s/%s?q=%s",
		DoltHubAPIBase, r.writeOwner, r.writeDB, url.PathEscape(branch), url.QueryEscape(sql))

	body, err := r.doGet(apiURL)
	if err != nil {
		return "", fmt.Errorf("queryForkBranch failed: %w", err)
	}

	return JSONToCSV(body)
}

// ensureUpstreamRemote checks if the "upstream" remote exists on the fork and
// adds it if missing.
func (r *RemoteDB) ensureUpstreamRemote() error {
	csv, err := r.queryForkMain("SELECT name FROM dolt_remotes WHERE name = 'upstream'")
	if err != nil {
		return err
	}

	// If we got rows beyond the header, the remote exists.
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) >= 2 {
		return nil
	}

	// Add the upstream remote.
	remoteURL := fmt.Sprintf("https://doltremoteapi.dolthub.com/%s/%s", r.readOwner, r.readDB)
	escaped := strings.ReplaceAll(remoteURL, "'", "''")
	sql := fmt.Sprintf("CALL DOLT_REMOTE('add', 'upstream', '%s')", escaped)
	return r.execOnMain(sql)
}

// parseDiffTables extracts table names from a dolt_diff CSV result.
func parseDiffTables(csv string) []string {
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) < 2 {
		return nil
	}
	var tables []string
	for _, line := range lines[1:] {
		parts := strings.SplitN(line, ",", 2)
		name := strings.TrimSpace(parts[0])
		if name != "" {
			tables = append(tables, name)
		}
	}
	return tables
}

// formatDiffRow formats a single diff row into a human-readable block.
// It pairs from_* and to_* columns to show changes.
func formatDiffRow(buf *strings.Builder, header, fields []string) {
	// Find diff_type column.
	diffType := ""
	id := ""
	for i, col := range header {
		if i >= len(fields) {
			break
		}
		if col == "diff_type" {
			diffType = fields[i]
		}
		if col == "to_id" && fields[i] != "" {
			id = fields[i]
		} else if col == "from_id" && id == "" && fields[i] != "" {
			id = fields[i]
		}
	}

	prefix := "~"
	switch diffType {
	case "added":
		prefix = "+"
	case "removed":
		prefix = "-"
	}
	fmt.Fprintf(buf, "%s %s: id=%s\n", prefix, diffType, id)

	// Show changed fields by pairing from_* and to_* columns.
	fromVals := map[string]string{}
	toVals := map[string]string{}
	for i, col := range header {
		if i >= len(fields) {
			break
		}
		if col == "diff_type" || col == "from_commit" || col == "to_commit" ||
			col == "from_commit_date" || col == "to_commit_date" {
			continue
		}
		if strings.HasPrefix(col, "from_") {
			fromVals[strings.TrimPrefix(col, "from_")] = fields[i]
		} else if strings.HasPrefix(col, "to_") {
			toVals[strings.TrimPrefix(col, "to_")] = fields[i]
		}
	}

	for field, fromVal := range fromVals {
		toVal := toVals[field]
		if fromVal != toVal {
			if fromVal == "" {
				fromVal = "(empty)"
			}
			if toVal == "" {
				toVal = "(empty)"
			}
			fmt.Fprintf(buf, "  %s: %s → %s\n", field, fromVal, toVal)
		}
	}
	// Show fields that only exist in to_ (new fields on added rows).
	for field, toVal := range toVals {
		if _, exists := fromVals[field]; !exists && toVal != "" {
			fmt.Fprintf(buf, "  %s: %s\n", field, toVal)
		}
	}
}

// --- HTTP helpers ---

func (r *RemoteDB) doGet(apiURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "token "+r.token)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func (r *RemoteDB) doPost(apiURL string, payload []byte) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest("POST", apiURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "token "+r.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// pollOperation polls a DoltHub async write operation until it completes.
func (r *RemoteDB) pollOperation(operationName string) error {
	backoff := 500 * time.Millisecond
	deadline := time.Now().Add(2 * time.Minute)

	for time.Now().Before(deadline) {
		time.Sleep(backoff)

		apiURL := fmt.Sprintf("%s/%s/%s/write?operationName=%s",
			DoltHubAPIBase, r.writeOwner, r.writeDB, operationName)

		body, err := r.doGet(apiURL)
		if err != nil {
			if backoff < 8*time.Second {
				backoff *= 2
			}
			continue
		}

		var pollResp struct {
			QueryExecutionStatus  string `json:"query_execution_status"`
			QueryExecutionMessage string `json:"query_execution_message"`
		}
		if err := json.Unmarshal(body, &pollResp); err == nil {
			status := strings.ToLower(pollResp.QueryExecutionStatus)
			if status == "success" || status == "successwithwarning" {
				return nil
			}
			if status == "error" {
				return fmt.Errorf("write operation failed: %s", pollResp.QueryExecutionMessage)
			}
		}

		if backoff < 8*time.Second {
			backoff *= 2
		}
	}

	return fmt.Errorf("timed out waiting for write operation %q", operationName)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
