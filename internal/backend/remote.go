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
	client     *http.Client
}

// NewRemoteDB creates a DB backed by the DoltHub REST API.
func NewRemoteDB(token, readOwner, readDB, writeOwner, writeDB string) *RemoteDB {
	return &RemoteDB{
		token:      token,
		readOwner:  readOwner,
		readDB:     readDB,
		writeOwner: writeOwner,
		writeDB:    writeDB,
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
		DoltHubAPIBase, owner, db, branch, url.QueryEscape(sql))

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

// Sync is a no-op for remote — reads always go to the API.
func (r *RemoteDB) Sync() error { return nil }

// MergeBranch is a no-op for remote — merges happen via DoltHub PRs.
func (r *RemoteDB) MergeBranch(_ string) error { return nil }

// DeleteRemoteBranch is a no-op for remote — DeleteBranch already removes the API branch.
func (r *RemoteDB) DeleteRemoteBranch(_ string) error { return nil }

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
