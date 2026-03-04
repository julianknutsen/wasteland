package hosted

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeForkRegistrar records calls and returns a configurable warning.
type fakeForkRegistrar struct {
	calls   []forkRegisterCall
	warning string // returned from EnsureForkAndRegister
}

type forkRegisterCall struct {
	APIKey      string
	Upstream    string
	ForkOrg     string
	ForkDB      string
	RigHandle   string
	DisplayName string
	Email       string
}

func (f *fakeForkRegistrar) EnsureForkAndRegister(apiKey, upstream, forkOrg, forkDB, rigHandle, displayName, email string) string {
	f.calls = append(f.calls, forkRegisterCall{
		APIKey:      apiKey,
		Upstream:    upstream,
		ForkOrg:     forkOrg,
		ForkDB:      forkDB,
		RigHandle:   rigHandle,
		DisplayName: displayName,
		Email:       email,
	})
	return f.warning
}

func TestHandleConnect_ForkRegistrar_HappyPath(t *testing.T) {
	fake := &fakeForkRegistrar{}
	server, ts := setupTestServer(t, "test-token", fake)
	_ = server
	defer ts.Close()

	body := `{
		"connection_id": "conn-1",
		"rig_handle": "alice",
		"fork_org": "alice-org",
		"fork_db": "wl-commons",
		"upstream": "hop/wl-commons",
		"display_name": "Alice",
		"email": "alice@example.com"
	}`
	resp, err := http.Post(ts.URL+"/api/auth/connect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	// Verify fork registrar was called.
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.calls))
	}
	call := fake.calls[0]
	if call.APIKey != "test-token" {
		t.Errorf("expected apiKey test-token, got %s", call.APIKey)
	}
	if call.RigHandle != "alice" {
		t.Errorf("expected handle alice, got %s", call.RigHandle)
	}
	if call.DisplayName != "Alice" {
		t.Errorf("expected display_name Alice, got %s", call.DisplayName)
	}

	// Response should not contain setup_warning.
	var result map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["setup_warning"]; ok {
		t.Error("expected no setup_warning in response")
	}
}

func TestHandleConnect_ForkRegistrar_Warning(t *testing.T) {
	fake := &fakeForkRegistrar{warning: "fork failed: timeout"}
	_, ts := setupTestServer(t, "test-token", fake)
	defer ts.Close()

	body := `{
		"connection_id": "conn-1",
		"rig_handle": "alice",
		"fork_org": "alice-org",
		"fork_db": "wl-commons",
		"upstream": "hop/wl-commons"
	}`
	resp, err := http.Post(ts.URL+"/api/auth/connect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Connect should still succeed even with fork warning.
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var result map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["setup_warning"] != "fork failed: timeout" {
		t.Errorf("expected setup_warning, got %q", result["setup_warning"])
	}
}

func TestHandleConnect_ForkRegistrar_NoAPIKey(t *testing.T) {
	fake := &fakeForkRegistrar{}
	_, ts := setupTestServer(t, "", fake) // empty API key
	defer ts.Close()

	body := `{
		"connection_id": "conn-1",
		"rig_handle": "alice",
		"fork_org": "alice-org",
		"fork_db": "wl-commons",
		"upstream": "hop/wl-commons"
	}`
	resp, err := http.Post(ts.URL+"/api/auth/connect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Still succeeds — fork registrar is called with empty key, it returns a warning.
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.calls))
	}
	if fake.calls[0].APIKey != "" {
		t.Errorf("expected empty apiKey, got %s", fake.calls[0].APIKey)
	}
}

func TestHandleJoin_ForkRegistrar_HappyPath(t *testing.T) {
	fake := &fakeForkRegistrar{}
	server, ts := setupTestServer(t, "test-token", fake)
	defer ts.Close()

	// Create session first.
	sessionID, _ := server.sessions.Create("conn-1")
	cookie := &http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, "session-secret"),
	}

	body := `{
		"fork_org": "bob-org",
		"fork_db": "wl-commons",
		"upstream": "hop/wl-commons",
		"display_name": "Bob",
		"email": "bob@example.com"
	}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.calls))
	}
	call := fake.calls[0]
	if call.ForkOrg != "bob-org" {
		t.Errorf("expected bob-org, got %s", call.ForkOrg)
	}
	if call.RigHandle != "alice" { // rig handle comes from existing metadata
		t.Errorf("expected alice (from metadata), got %s", call.RigHandle)
	}
}

func TestHandleJoin_ForkRegistrar_Warning(t *testing.T) {
	fake := &fakeForkRegistrar{warning: "registration failed"}
	server, ts := setupTestServer(t, "test-token", fake)
	defer ts.Close()

	sessionID, _ := server.sessions.Create("conn-1")
	cookie := &http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, "session-secret"),
	}

	body := `{
		"fork_org": "bob-org",
		"fork_db": "wl-commons",
		"upstream": "hop/wl-commons"
	}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Join still succeeds.
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var result map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["setup_warning"] != "registration failed" {
		t.Errorf("expected setup_warning, got %q", result["setup_warning"])
	}
}

// setupTestServer creates a Server with a fake Nango backend and the given fork registrar.
func setupTestServer(t *testing.T, apiKey string, registrar ForkRegistrar) (*Server, *httptest.Server) {
	t.Helper()

	meta := &UserMetadata{
		RigHandle: "alice",
		Wastelands: []WastelandConfig{
			{
				Upstream: "hop/wl-commons",
				ForkOrg:  "alice-org",
				ForkDB:   "wl-commons",
				Mode:     "pr",
			},
		},
	}

	nangoTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/connection/"):
			resp := nangoConnectionResponse{ConnectionID: "conn-1"}
			resp.Credentials.APIKey = apiKey
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
	t.Cleanup(nangoTS.Close)

	nango := NewNangoClient(NangoConfig{
		BaseURL:       nangoTS.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewWorkspaceResolver(nango, sessions)
	server := NewServer(resolver, sessions, nango, "session-secret")
	server.forkRegistrar = registrar

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/connect", server.handleConnect)
	mux.HandleFunc("POST /api/auth/join", server.handleJoin)

	ts := httptest.NewServer(mux)
	return server, ts
}
