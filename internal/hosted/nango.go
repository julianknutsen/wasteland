package hosted

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// NangoConfig holds configuration for the Nango integration.
type NangoConfig struct {
	BaseURL       string // default "https://api.nango.dev"
	SecretKey     string // server-side only
	IntegrationID string // default "dolthub"
}

// UserConfig is the persistent wasteland config stored as Nango connection metadata.
type UserConfig struct {
	RigHandle string `json:"rig_handle"`
	ForkOrg   string `json:"fork_org"`
	ForkDB    string `json:"fork_db"`
	Upstream  string `json:"upstream"`
	Mode      string `json:"mode"`    // "wild-west" or "pr"
	Signing   bool   `json:"signing"` // GPG-signed dolt commits
}

// NangoClient talks to the Nango REST API.
type NangoClient struct {
	baseURL       string
	secretKey     string
	integrationID string
	client        *http.Client
}

// NewNangoClient creates a NangoClient from the given config.
func NewNangoClient(cfg NangoConfig) *NangoClient {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.nango.dev"
	}
	integrationID := cfg.IntegrationID
	if integrationID == "" {
		integrationID = "dolthub"
	}
	return &NangoClient{
		baseURL:       base,
		secretKey:     cfg.SecretKey,
		integrationID: integrationID,
		client:        &http.Client{},
	}
}

// IntegrationID returns the configured Nango integration ID.
func (n *NangoClient) IntegrationID() string { return n.integrationID }

// BaseURL returns the Nango API base URL.
func (n *NangoClient) BaseURL() string { return n.baseURL }

// SecretKey returns the Nango server-side secret key.
func (n *NangoClient) SecretKey() string { return n.secretKey }

// nangoConnectionResponse is the JSON shape returned by GET /connection/{id}.
type nangoConnectionResponse struct {
	ConnectionID string `json:"connection_id"`
	Credentials  struct {
		APIKey string `json:"apiKey"`
	} `json:"credentials"`
	Metadata json.RawMessage `json:"metadata"`
}

// GetConnection fetches the stored token and metadata for a Nango connection.
func (n *NangoClient) GetConnection(connectionID string) (string, *UserConfig, error) {
	u := fmt.Sprintf("%s/connection/%s?provider_config_key=%s",
		n.baseURL, url.PathEscape(connectionID), url.QueryEscape(n.integrationID))

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.secretKey)

	resp, err := n.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("nango request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("nango returned %d: %s", resp.StatusCode, string(body))
	}

	var connResp nangoConnectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&connResp); err != nil {
		return "", nil, fmt.Errorf("decoding nango response: %w", err)
	}

	apiKey := connResp.Credentials.APIKey

	var cfg *UserConfig
	if len(connResp.Metadata) > 0 && string(connResp.Metadata) != "null" {
		cfg = &UserConfig{}
		if err := json.Unmarshal(connResp.Metadata, cfg); err != nil {
			cfg = nil // metadata is not our format, ignore
		}
	}

	return apiKey, cfg, nil
}

// connectSessionRequest is the JSON body for POST /connect/sessions.
type connectSessionAPIRequest struct {
	EndUser             connectSessionEndUser `json:"end_user"`
	AllowedIntegrations []string              `json:"allowed_integrations"`
}

type connectSessionEndUser struct {
	ID string `json:"id"`
}

// connectSessionAPIResponse is the JSON shape returned by POST /connect/sessions.
type connectSessionAPIResponse struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

// CreateConnectSession creates a short-lived connect session token for the frontend SDK.
func (n *NangoClient) CreateConnectSession(endUserID string) (string, error) {
	u := fmt.Sprintf("%s/connect/sessions", n.baseURL)

	body, err := json.Marshal(connectSessionAPIRequest{
		EndUser:             connectSessionEndUser{ID: endUserID},
		AllowedIntegrations: []string{n.integrationID},
	})
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("nango request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("nango returned %d: %s", resp.StatusCode, string(respBody))
	}

	var sessionResp connectSessionAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return "", fmt.Errorf("decoding nango response: %w", err)
	}

	return sessionResp.Data.Token, nil
}

// SetMetadata writes/updates the persistent user config on the Nango connection.
func (n *NangoClient) SetMetadata(connectionID string, cfg *UserConfig) error {
	u := fmt.Sprintf("%s/connection/%s/metadata",
		n.baseURL, url.PathEscape(connectionID))

	body, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	req, err := http.NewRequest("PATCH", u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.secretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Provider-Config-Key", n.integrationID)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("nango request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nango returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
