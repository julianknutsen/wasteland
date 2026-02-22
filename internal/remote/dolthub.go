package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// dolthubAPIBase is the DoltHub REST API base URL.
// Var so tests can override it.
var dolthubAPIBase = "https://www.dolthub.com/api/v1alpha1"

const dolthubRemoteBase = "https://doltremoteapi.dolthub.com"

// DoltHubProvider implements Provider for DoltHub-hosted databases.
type DoltHubProvider struct {
	token string
}

// NewDoltHubProvider creates a DoltHubProvider with the given API token.
func NewDoltHubProvider(token string) *DoltHubProvider {
	return &DoltHubProvider{token: token}
}

func (d *DoltHubProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("%s/%s/%s", dolthubRemoteBase, org, db)
}

func (d *DoltHubProvider) Fork(fromOrg, fromDB, toOrg string) error {
	body := map[string]string{
		"owner_name":     toOrg,
		"new_repo_name":  fromDB,
		"from_owner":     fromOrg,
		"from_repo_name": fromDB,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling fork request: %w", err)
	}

	url := dolthubAPIBase + "/database/fork"
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating fork request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", "token "+d.token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("DoltHub fork API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil {
		if strings.Contains(strings.ToLower(errResp.Message), "already exists") {
			return nil
		}
		return fmt.Errorf("DoltHub fork API error (HTTP %d): %s", resp.StatusCode, errResp.Message)
	}
	return fmt.Errorf("DoltHub fork API error (HTTP %d)", resp.StatusCode)
}

func (d *DoltHubProvider) Type() string { return "dolthub" }
