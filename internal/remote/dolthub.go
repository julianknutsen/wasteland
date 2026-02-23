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
// support). Otherwise checks if the fork already exists on DoltHub — if it
// does, continues silently; if not, returns a ForkRequiredError with
// instructions for the user to fork manually on dolthub.com.
func (d *DoltHubProvider) Fork(fromOrg, fromDB, toOrg string) error {
	sessionToken := os.Getenv("DOLTHUB_SESSION_TOKEN")
	if sessionToken != "" {
		return d.forkGraphQL(fromOrg, fromDB, toOrg, sessionToken)
	}

	// No session token — check if fork already exists.
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

// Type returns "dolthub".
func (d *DoltHubProvider) Type() string { return "dolthub" }
