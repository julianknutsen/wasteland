package hosted

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/api"
)

// TestHostedEndToEnd exercises the full flow: auth middleware -> api.Server routes
// via NewHostedWorkspace + NewClientFunc + NewWorkspaceFunc. This ensures the
// context injection actually bridges to real API handlers.
func TestHostedEndToEnd(t *testing.T) {
	meta := &UserMetadata{
		RigHandle: "alice",
		Wastelands: []WastelandConfig{
			{
				Upstream: "wasteland/wl-commons",
				ForkOrg:  "alice-org",
				ForkDB:   "wl-commons",
				Mode:     "wild-west",
			},
		},
	}

	nangoTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/connection/conn-1":
			resp := nangoConnectionResponse{ConnectionID: "conn-1"}
			resp.Credentials.APIKey = "test-token"
			b, _ := json.Marshal(meta)
			resp.Metadata = json.RawMessage(b)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/connection/"):
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer nangoTS.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       nangoTS.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewWorkspaceResolver(nango, sessions)
	hostedServer := NewServer(resolver, sessions, nango, "session-secret")

	// Use the real api.NewHostedWorkspace with NewClientFunc + NewWorkspaceFunc.
	apiServer := api.NewHostedWorkspace(NewClientFunc(), NewWorkspaceFunc())

	// Use an empty FS since we only test API routes.
	handler := hostedServer.Handler(apiServer, emptyFS{})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 1. Unauthenticated request to /api/config should return 401.
	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated, got %d", resp.StatusCode)
	}

	// 2. Connect and create a session.
	connectBody := `{
		"connection_id": "conn-1",
		"rig_handle": "alice",
		"fork_org": "alice-org",
		"fork_db": "wl-commons",
		"upstream": "wasteland/wl-commons"
	}`
	connectResp, err := http.Post(ts.URL+"/api/auth/connect", "application/json", strings.NewReader(connectBody))
	if err != nil {
		t.Fatal(err)
	}
	defer connectResp.Body.Close() //nolint:errcheck // test cleanup
	if connectResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(connectResp.Body)
		t.Fatalf("connect failed: %d %s", connectResp.StatusCode, string(body))
	}

	// Extract session cookie.
	var sessionCookie *http.Cookie
	for _, c := range connectResp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie in connect response")
	}

	// 3. Authenticated request to /api/config should succeed and return hosted fields.
	// Single wasteland: X-Wasteland header is optional.
	req, _ := http.NewRequest("GET", ts.URL+"/api/config", nil)
	req.AddCookie(sessionCookie)

	configResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer configResp.Body.Close() //nolint:errcheck // test cleanup
	if configResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(configResp.Body)
		t.Fatalf("config failed: %d %s", configResp.StatusCode, string(body))
	}

	var configResult api.ConfigResponse
	if err := json.NewDecoder(configResp.Body).Decode(&configResult); err != nil {
		t.Fatalf("decoding config: %v", err)
	}
	if configResult.RigHandle != "alice" {
		t.Errorf("expected alice, got %s", configResult.RigHandle)
	}
	if configResult.Mode != "wild-west" {
		t.Errorf("expected wild-west, got %s", configResult.Mode)
	}
	if !configResult.Hosted {
		t.Error("expected Hosted=true")
	}
	if !configResult.Connected {
		t.Error("expected Connected=true")
	}
	// Should include upstreams list.
	if len(configResult.Upstreams) != 1 {
		t.Errorf("expected 1 upstream in config, got %d", len(configResult.Upstreams))
	}
}

// TestConfigNotHosted verifies that self-sovereign mode does NOT set hosted fields.
func TestConfigNotHosted(t *testing.T) {
	meta := &UserMetadata{
		RigHandle: "bob",
		Wastelands: []WastelandConfig{
			{
				Upstream: "wasteland/wl-commons",
				ForkOrg:  "bob-org",
				ForkDB:   "wl-commons",
				Mode:     "pr",
			},
		},
	}

	nangoTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/connection/conn-1" {
			resp := nangoConnectionResponse{ConnectionID: "conn-1"}
			resp.Credentials.APIKey = "test-token"
			b, _ := json.Marshal(meta)
			resp.Metadata = json.RawMessage(b)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer nangoTS.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       nangoTS.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewWorkspaceResolver(nango, sessions)

	// Use NewWithClientFunc (self-sovereign style) -- NOT NewHosted.
	apiServer := api.NewWithClientFunc(NewClientFunc())

	hostedServer := NewServer(resolver, sessions, nango, "session-secret")
	handler := hostedServer.Handler(apiServer, emptyFS{})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Create a session.
	sessionID := sessions.Create("conn-1")
	req, _ := http.NewRequest("GET", ts.URL+"/api/config", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, "session-secret"),
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var configResult api.ConfigResponse
	_ = json.NewDecoder(resp.Body).Decode(&configResult)

	if configResult.Hosted {
		t.Error("expected Hosted=false for non-hosted server")
	}
	if configResult.Connected {
		t.Error("expected Connected=false for non-hosted server")
	}
}

// TestResolverSaveConfigWritesToNango verifies the SaveConfig callback on
// a resolver-built client actually writes to Nango metadata.
func TestResolverSaveConfigWritesToNango(t *testing.T) {
	var savedBody []byte

	meta := &UserMetadata{
		RigHandle: "alice",
		Wastelands: []WastelandConfig{
			{
				Upstream: "wasteland/wl-commons",
				ForkOrg:  "alice-org",
				ForkDB:   "wl-commons",
				Mode:     "wild-west",
			},
		},
	}

	nangoTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/connection/conn-1":
			resp := nangoConnectionResponse{ConnectionID: "conn-1"}
			resp.Credentials.APIKey = "test-token"
			b, _ := json.Marshal(meta)
			resp.Metadata = json.RawMessage(b)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/connection/"):
			savedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer nangoTS.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       nangoTS.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewWorkspaceResolver(nango, sessions)

	session := &UserSession{ID: "sess-1", ConnectionID: "conn-1"}
	ws, err := resolver.Resolve(session)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	client, err := ws.Client("wasteland/wl-commons")
	if err != nil {
		t.Fatalf("get client: %v", err)
	}

	// Call SaveSettings which triggers the SaveConfig callback.
	if err := client.SaveSettings("pr", true); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	// Verify the metadata was written to Nango in new format.
	if savedBody == nil {
		t.Fatal("expected SetMetadata to be called")
	}
	var savedMeta UserMetadata
	if err := json.Unmarshal(savedBody, &savedMeta); err != nil {
		t.Fatalf("decoding saved metadata: %v", err)
	}
	if savedMeta.RigHandle != "alice" {
		t.Errorf("expected alice, got %s", savedMeta.RigHandle)
	}
	if len(savedMeta.Wastelands) != 1 {
		t.Fatalf("expected 1 wasteland, got %d", len(savedMeta.Wastelands))
	}
	wl := savedMeta.Wastelands[0]
	if wl.Mode != "pr" {
		t.Errorf("expected mode pr, got %s", wl.Mode)
	}
	if !wl.Signing {
		t.Error("expected signing=true")
	}
}

// emptyFS implements fs.FS with no files, used for testing API-only scenarios.
type emptyFS struct{}

func (emptyFS) Open(_ string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: "", Err: fs.ErrNotExist}
}
