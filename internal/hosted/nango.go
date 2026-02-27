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

// WastelandConfig describes a single joined wasteland in Nango metadata.
type WastelandConfig struct {
	Upstream string `json:"upstream"`
	ForkOrg  string `json:"fork_org"`
	ForkDB   string `json:"fork_db"`
	Mode     string `json:"mode"`    // "wild-west" or "pr"
	Signing  bool   `json:"signing"` // GPG-signed dolt commits
}

// UserMetadata is the persistent user config stored as Nango connection metadata.
type UserMetadata struct {
	RigHandle  string            `json:"rig_handle"`
	Wastelands []WastelandConfig `json:"wastelands"`
}

// FindWasteland returns the config for the given upstream, or nil if not found.
func (m *UserMetadata) FindWasteland(upstream string) *WastelandConfig {
	for i := range m.Wastelands {
		if m.Wastelands[i].Upstream == upstream {
			return &m.Wastelands[i]
		}
	}
	return nil
}

// UpsertWasteland adds or updates a wasteland entry.
func (m *UserMetadata) UpsertWasteland(wl WastelandConfig) {
	for i := range m.Wastelands {
		if m.Wastelands[i].Upstream == wl.Upstream {
			m.Wastelands[i] = wl
			return
		}
	}
	m.Wastelands = append(m.Wastelands, wl)
}

// RemoveWasteland removes the wasteland with the given upstream.
// Returns false if the upstream was not found.
func (m *UserMetadata) RemoveWasteland(upstream string) bool {
	for i := range m.Wastelands {
		if m.Wastelands[i].Upstream == upstream {
			m.Wastelands = append(m.Wastelands[:i], m.Wastelands[i+1:]...)
			return true
		}
	}
	return false
}

// UserConfig is the legacy flat metadata format. Kept for backward compatibility
// parsing only — new writes always use UserMetadata.
type UserConfig struct {
	RigHandle string `json:"rig_handle"`
	ForkOrg   string `json:"fork_org"`
	ForkDB    string `json:"fork_db"`
	Upstream  string `json:"upstream"`
	Mode      string `json:"mode"`
	Signing   bool   `json:"signing"`
}

// parseMetadata reads Nango metadata JSON and returns a UserMetadata.
// It handles both the new multi-wasteland format and the legacy flat format.
func parseMetadata(raw json.RawMessage) *UserMetadata {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	// Try new format first (has "wastelands" array).
	var meta UserMetadata
	if err := json.Unmarshal(raw, &meta); err == nil && len(meta.Wastelands) > 0 {
		return &meta
	}

	// Try legacy flat format (has top-level "upstream" field).
	var legacy UserConfig
	if err := json.Unmarshal(raw, &legacy); err == nil && legacy.Upstream != "" {
		return &UserMetadata{
			RigHandle: legacy.RigHandle,
			Wastelands: []WastelandConfig{
				{
					Upstream: legacy.Upstream,
					ForkOrg:  legacy.ForkOrg,
					ForkDB:   legacy.ForkDB,
					Mode:     legacy.Mode,
					Signing:  legacy.Signing,
				},
			},
		}
	}

	return nil
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
func (n *NangoClient) GetConnection(connectionID string) (string, *UserMetadata, error) {
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
	meta := parseMetadata(connResp.Metadata)

	return apiKey, meta, nil
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

// SetMetadata writes/updates the persistent user metadata on the Nango connection.
func (n *NangoClient) SetMetadata(connectionID string, meta *UserMetadata) error {
	u := fmt.Sprintf("%s/connection/%s/metadata",
		n.baseURL, url.PathEscape(connectionID))

	body, err := json.Marshal(meta)
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
