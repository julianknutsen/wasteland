package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// dolthubGraphQLURL is the DoltHub GraphQL API endpoint. Var so tests can override.
var dolthubGraphQLURL = "https://www.dolthub.com/graphql"

// dolthubAPIBase is the DoltHub REST API base URL. Var so tests can override.
var dolthubAPIBase = "https://www.dolthub.com/api/v1alpha1"

const (
	dolthubRemoteBase = "https://doltremoteapi.dolthub.com"
	dolthubRepoBase   = "https://www.dolthub.com/repositories"
)

// DoltHubProvider implements Provider for DoltHub-hosted databases.
type DoltHubProvider struct {
	token string
}

// NewDoltHubProvider creates a DoltHubProvider with the given API token.
func NewDoltHubProvider(token string) *DoltHubProvider {
	return &DoltHubProvider{token: token}
}

// ForkRequiredError is returned when the user needs to manually fork on DoltHub.
type ForkRequiredError struct {
	UpstreamOrg string
	UpstreamDB  string
	ForkOrg     string
}

func (e *ForkRequiredError) Error() string {
	return fmt.Sprintf("fork %s/%s not found under %s on DoltHub", e.UpstreamOrg, e.UpstreamDB, e.ForkOrg)
}

// ForkURL returns the DoltHub URL where the user can fork the database.
func (e *ForkRequiredError) ForkURL() string {
	return fmt.Sprintf("%s/%s/%s", dolthubRepoBase, e.UpstreamOrg, e.UpstreamDB)
}

// DatabaseURL returns the DoltHub remote API URL for the given org/db.
func (d *DoltHubProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("%s/%s/%s", dolthubRemoteBase, org, db)
}

// Fork creates a fork of fromOrg/fromDB under toOrg on DoltHub.
//
// If DOLTHUB_SESSION_TOKEN is set (browser session cookie), uses the GraphQL
// createFork mutation which preserves DoltHub fork metadata (parent link, PR
// support). Otherwise attempts the REST API fork endpoint using the standard
// DOLTHUB_TOKEN. If the REST API fails due to auth/permission errors, falls
// back to checking if the fork already exists and returns a ForkRequiredError
// if not.
func (d *DoltHubProvider) Fork(fromOrg, fromDB, toOrg string) error {
	sessionToken := os.Getenv("DOLTHUB_SESSION_TOKEN")
	if sessionToken != "" {
		return d.forkGraphQL(fromOrg, fromDB, toOrg, sessionToken)
	}
	return d.forkREST(fromOrg, fromDB, toOrg)
}

// forkREST uses the DoltHub REST API to create a fork. It POSTs to the fork
// endpoint and polls until the operation completes. Falls back to an
// exists-check with ForkRequiredError if the API returns an auth error.
func (d *DoltHubProvider) forkREST(fromOrg, fromDB, toOrg string) error {
	reqBody, err := json.Marshal(map[string]string{
		"ownerName":          toOrg,
		"parentOwnerName":    fromOrg,
		"parentDatabaseName": fromDB,
	})
	if err != nil {
		return fmt.Errorf("marshaling REST fork request: %w", err)
	}

	req, err := http.NewRequest("POST", dolthubAPIBase+"/fork", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("creating REST fork request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", "token "+d.token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return d.forkRESTFallback(fromOrg, fromDB, toOrg,
			fmt.Errorf("REST fork request failed: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading REST fork response: %w", err)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return d.forkRESTFallback(fromOrg, fromDB, toOrg,
			fmt.Errorf("REST fork auth error (HTTP %d): %s", resp.StatusCode, string(body)))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Check for "already exists" in error responses.
		if strings.Contains(strings.ToLower(string(body)), "already exists") {
			return nil
		}
		return d.forkRESTFallback(fromOrg, fromDB, toOrg,
			fmt.Errorf("REST fork error (HTTP %d): %s", resp.StatusCode, string(body)))
	}

	var forkResp struct {
		Status        string `json:"status"`
		OperationName string `json:"operation_name"`
	}
	if err := json.Unmarshal(body, &forkResp); err != nil {
		return fmt.Errorf("parsing REST fork response: %w", err)
	}

	// If the response already has a success status with no operation to poll, we're done.
	if forkResp.OperationName == "" {
		return nil
	}

	// Poll until the fork operation completes.
	return d.pollForkOperation(forkResp.OperationName, toOrg, fromDB)
}

// pollForkOperation polls the fork endpoint until the operation completes.
// Falls back to checking if the database exists when the poll times out,
// since the fork may complete with a response format we don't recognize.
func (d *DoltHubProvider) pollForkOperation(operationName, forkOrg, forkDB string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	backoff := 500 * time.Millisecond
	deadline := time.Now().Add(2 * time.Minute)

	for time.Now().Before(deadline) {
		time.Sleep(backoff)

		url := fmt.Sprintf("%s/fork?operationName=%s", dolthubAPIBase, operationName)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("creating fork poll request: %w", err)
		}
		req.Header.Set("authorization", "token "+d.token)

		resp, err := client.Do(req)
		if err != nil {
			if backoff < 8*time.Second {
				backoff *= 2
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			if backoff < 8*time.Second {
				backoff *= 2
			}
			continue
		}

		var pollResp struct {
			Status       string `json:"status"`
			OwnerName    string `json:"owner_name"`
			DatabaseName string `json:"database_name"`
		}
		if err := json.Unmarshal(body, &pollResp); err == nil {
			// Check for completion via owner_name + database_name fields.
			if pollResp.OwnerName != "" && pollResp.DatabaseName != "" {
				return nil
			}
			// Check for a terminal status field (e.g., "Success", "Done").
			s := strings.ToLower(pollResp.Status)
			if s == "success" || s == "done" || s == "completed" {
				return nil
			}
		}

		if backoff < 8*time.Second {
			backoff *= 2
		}
	}

	// The poll timed out, but the fork may have completed anyway.
	// Check directly whether the database exists before reporting failure.
	if d.databaseExists(forkOrg, forkDB) {
		return nil
	}

	return fmt.Errorf("timed out waiting for fork operation %q to complete", operationName)
}

// forkRESTFallback falls back to the exists-check when the REST API fork fails.
func (d *DoltHubProvider) forkRESTFallback(fromOrg, fromDB, toOrg string, _ error) error {
	if d.databaseExists(toOrg, fromDB) {
		return nil
	}
	return &ForkRequiredError{
		UpstreamOrg: fromOrg,
		UpstreamDB:  fromDB,
		ForkOrg:     toOrg,
	}
}

// forkGraphQL uses the DoltHub GraphQL createFork mutation with a browser
// session cookie. This preserves fork metadata on DoltHub.
func (d *DoltHubProvider) forkGraphQL(fromOrg, fromDB, toOrg, sessionToken string) error {
	query := `mutation CreateFork($ownerName: String!, $parentOwnerName: String!, $parentRepoName: String!) {
  createFork(ownerName: $ownerName, parentOwnerName: $parentOwnerName, parentRepoName: $parentRepoName) {
    forkOperationName
  }
}`
	reqBody := graphqlRequest{
		Query: query,
		Variables: map[string]any{
			"ownerName":       toOrg,
			"parentOwnerName": fromOrg,
			"parentRepoName":  fromDB,
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling fork request: %w", err)
	}

	req, err := http.NewRequest("POST", dolthubGraphQLURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating fork request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "dolthubToken="+sessionToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("DoltHub GraphQL fork request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading DoltHub GraphQL response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("DoltHub GraphQL error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return fmt.Errorf("parsing DoltHub GraphQL response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		msg := gqlResp.Errors[0].Message
		if strings.Contains(strings.ToLower(msg), "already exists") ||
			strings.Contains(strings.ToLower(msg), "already been forked") {
			return nil
		}
		return fmt.Errorf("DoltHub GraphQL fork error: %s", msg)
	}

	return nil
}

// databaseExists checks if a database exists on DoltHub by querying the
// REST API. Returns true if the API returns HTTP 200 for the main branch.
func (d *DoltHubProvider) databaseExists(org, db string) bool {
	url := fmt.Sprintf("%s/%s/%s/main", dolthubAPIBase, org, db)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("authorization", "token "+d.token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == 200
}

// graphqlRequest is the JSON body sent to the GraphQL endpoint.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphqlResponse is the top-level JSON response from GraphQL.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// CreatePR opens a pull request on DoltHub from forkOrg/db (fromBranch) to upstreamOrg/db (main).
func (d *DoltHubProvider) CreatePR(forkOrg, upstreamOrg, db, fromBranch, title, body string) (string, error) {
	url := fmt.Sprintf("%s/%s/%s/pulls", dolthubAPIBase, upstreamOrg, db)
	reqBody, err := json.Marshal(map[string]string{
		"title":               title,
		"description":         body,
		"fromBranchOwnerName": forkOrg,
		"fromBranchRepoName":  db,
		"fromBranchName":      fromBranch,
		"toBranchOwnerName":   upstreamOrg,
		"toBranchRepoName":    db,
		"toBranchName":        "main",
	})
	if err != nil {
		return "", fmt.Errorf("marshaling PR request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("creating PR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", "token "+d.token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DoltHub create PR request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading DoltHub PR response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("DoltHub create PR error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Extract PR URL from response. The API returns the PR status with an ID.
	var prResp struct {
		Status string `json:"status"`
		ID     string `json:"_id"`
	}
	if err := json.Unmarshal(respBody, &prResp); err == nil && prResp.ID != "" {
		return fmt.Sprintf("%s/%s/%s/pulls/%s", dolthubRepoBase, upstreamOrg, db, prResp.ID), nil
	}

	// Fallback: return the pulls page URL.
	return fmt.Sprintf("%s/%s/%s/pulls", dolthubRepoBase, upstreamOrg, db), nil
}

// FindPR searches for an open PR on DoltHub matching the given from-branch and fork org.
// Returns the PR URL and ID, or empty strings if none found.
//
// The list endpoint doesn't include branch details, so we fetch each open PR's
// detail to match on from_branch and from_branch_owner.
func (d *DoltHubProvider) FindPR(upstreamOrg, db, forkOrg, fromBranch string) (prURL, prID string) {
	// List all PRs.
	listURL := fmt.Sprintf("%s/%s/%s/pulls", dolthubAPIBase, upstreamOrg, db)
	body, err := d.dolthubGet(listURL)
	if err != nil {
		return "", ""
	}

	var result struct {
		Pulls []struct {
			PullID string `json:"pull_id"`
			State  string `json:"state"`
		} `json:"pulls"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", ""
	}

	// Check each open PR's detail for matching branch info.
	for _, pr := range result.Pulls {
		if !strings.EqualFold(pr.State, "open") {
			continue
		}
		detailURL := fmt.Sprintf("%s/%s/%s/pulls/%s", dolthubAPIBase, upstreamOrg, db, pr.PullID)
		detail, err := d.dolthubGet(detailURL)
		if err != nil {
			continue
		}
		var prDetail struct {
			FromBranch      string `json:"from_branch"`
			FromBranchOwner string `json:"from_branch_owner"`
		}
		if err := json.Unmarshal(detail, &prDetail); err != nil {
			continue
		}
		if prDetail.FromBranch == fromBranch && prDetail.FromBranchOwner == forkOrg {
			url := fmt.Sprintf("%s/%s/%s/pulls/%s", dolthubRepoBase, upstreamOrg, db, pr.PullID)
			return url, pr.PullID
		}
	}
	return "", ""
}

// dolthubGet performs an authenticated GET request to the DoltHub API.
func (d *DoltHubProvider) dolthubGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", d.token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// UpdatePR updates the title and description of an existing DoltHub pull request.
func (d *DoltHubProvider) UpdatePR(upstreamOrg, db, prID, title, description string) error {
	patchURL := fmt.Sprintf("%s/%s/%s/pulls/%s", dolthubAPIBase, upstreamOrg, db, prID)
	reqBody, err := json.Marshal(map[string]string{
		"title":       title,
		"description": description,
	})
	if err != nil {
		return fmt.Errorf("marshaling PR update: %w", err)
	}

	req, err := http.NewRequest("PATCH", patchURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("creating PR update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", d.token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("DoltHub update PR request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DoltHub update PR error (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// ListPendingWantedIDs returns a set of wanted IDs that have open upstream PRs.
// It lists open PRs on the upstream repo, fetches each PR's detail to get the
// from_branch, and extracts the wanted ID from the wl/{rig}/{wantedID} convention.
func (d *DoltHubProvider) ListPendingWantedIDs(upstreamOrg, db string) (map[string]bool, error) {
	listURL := fmt.Sprintf("%s/%s/%s/pulls", dolthubAPIBase, upstreamOrg, db)
	body, err := d.dolthubGet(listURL)
	if err != nil {
		return nil, fmt.Errorf("listing PRs: %w", err)
	}

	var result struct {
		Pulls []struct {
			PullID string `json:"pull_id"`
			State  string `json:"state"`
		} `json:"pulls"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing PR list: %w", err)
	}

	ids := make(map[string]bool)
	for _, pr := range result.Pulls {
		if !strings.EqualFold(pr.State, "open") {
			continue
		}
		detailURL := fmt.Sprintf("%s/%s/%s/pulls/%s", dolthubAPIBase, upstreamOrg, db, pr.PullID)
		detail, err := d.dolthubGet(detailURL)
		if err != nil {
			continue
		}
		var prDetail struct {
			FromBranch string `json:"from_branch"`
		}
		if err := json.Unmarshal(detail, &prDetail); err != nil {
			continue
		}
		// Extract wanted ID from wl/{rig}/{wantedID} convention.
		if !strings.HasPrefix(prDetail.FromBranch, "wl/") {
			continue
		}
		rest := strings.TrimPrefix(prDetail.FromBranch, "wl/")
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			continue
		}
		wantedID := rest[slashIdx+1:]
		if wantedID != "" {
			ids[wantedID] = true
		}
	}
	return ids, nil
}

// Type returns "dolthub".
func (d *DoltHubProvider) Type() string { return "dolthub" }
