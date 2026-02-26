package hosted

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSecret = "test-session-secret"

func setupHostedTestServer(t *testing.T) (*SessionStore, *httptest.Server) {
	t.Helper()

	cfg := &UserConfig{
		RigHandle: "alice",
		ForkOrg:   "alice-org",
		ForkDB:    "wl-commons",
		Upstream:  "wasteland/wl-commons",
		Mode:      "wild-west",
	}

	// Fake Nango server.
	nangoTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/connection/conn-1":
			resp := nangoConnectionResponse{ConnectionID: "conn-1"}
			resp.Credentials.APIKey = "test-token"
			b, _ := json.Marshal(cfg)
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
		SecretKey:     "nango-secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewClientResolver(nango, sessions)
	server := NewServer(resolver, sessions, nango, "pub-key-123", testSecret)

	// Create a simple test handler for the auth middleware.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For API routes, check that client is in context.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			_, ok := ClientFromContext(r.Context())
			if ok {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"no client in context"}`))
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("static"))
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/connect", server.handleConnect)
	mux.HandleFunc("GET /api/auth/status", server.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/logout", server.handleLogout)
	mux.HandleFunc("GET /api/auth/nango-key", server.handleNangoKey)
	mux.Handle("/", server.AuthMiddleware(inner))

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return sessions, ts
}

func TestAuthMiddleware_NoSession(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	resp, err := http.Get(ts.URL + "/api/wanted")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_ValidSession(t *testing.T) {
	sessions, ts := setupHostedTestServer(t)

	sessionID := sessions.Create("conn-1")

	req, _ := http.NewRequest("GET", ts.URL+"/api/wanted", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, testSecret),
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestAuthMiddleware_ExpiredSession(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	// Use a session ID that doesn't exist in the store.
	req, _ := http.NewRequest("GET", ts.URL+"/api/wanted", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: SignSessionID("nonexistent", testSecret),
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_NoConnectionID(t *testing.T) {
	sessions, ts := setupHostedTestServer(t)

	// Create session without connection ID.
	sessionID := sessions.Create("")

	req, _ := http.NewRequest("GET", ts.URL+"/api/wanted", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, testSecret),
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_StaticRoutes(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	// Static routes should not require auth.
	resp, err := http.Get(ts.URL + "/some-page")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for static route, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_AuthRoutes(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	// Auth routes should not require auth.
	resp, err := http.Get(ts.URL + "/api/auth/nango-key")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for auth route, got %d", resp.StatusCode)
	}
}

func TestHandleConnect(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	body := `{
		"connection_id": "conn-1",
		"rig_handle": "alice",
		"fork_org": "alice-org",
		"fork_db": "wl-commons",
		"upstream": "wasteland/wl-commons"
	}`
	resp, err := http.Post(ts.URL+"/api/auth/connect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Check that a session cookie was set.
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == cookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected wl_session cookie in response")
	}
}

func TestHandleConnect_MissingFields(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	body := `{"connection_id": "conn-1"}`
	resp, err := http.Post(ts.URL+"/api/auth/connect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleAuthStatus_NotAuthenticated(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	resp, err := http.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var status authStatusResponse
	_ = json.NewDecoder(resp.Body).Decode(&status)
	if status.Authenticated {
		t.Error("expected not authenticated")
	}
}

func TestHandleAuthStatus_Authenticated(t *testing.T) {
	sessions, ts := setupHostedTestServer(t)

	sessionID := sessions.Create("conn-1")

	req, _ := http.NewRequest("GET", ts.URL+"/api/auth/status", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, testSecret),
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var status authStatusResponse
	_ = json.NewDecoder(resp.Body).Decode(&status)
	if !status.Authenticated {
		t.Error("expected authenticated")
	}
	if !status.Connected {
		t.Error("expected connected")
	}
	if status.Config == nil {
		t.Error("expected config")
	}
}

func TestHandleLogout(t *testing.T) {
	sessions, ts := setupHostedTestServer(t)

	sessionID := sessions.Create("conn-1")

	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: SignSessionID(sessionID, testSecret),
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Session should be deleted.
	_, ok := sessions.Get(sessionID)
	if ok {
		t.Error("expected session to be deleted after logout")
	}
}

func TestHandleNangoKey(t *testing.T) {
	_, ts := setupHostedTestServer(t)

	resp, err := http.Get(ts.URL + "/api/auth/nango-key")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var key nangoKeyResponse
	_ = json.NewDecoder(resp.Body).Decode(&key)
	if key.PublicKey != "pub-key-123" {
		t.Errorf("expected pub-key-123, got %s", key.PublicKey)
	}
	if key.IntegrationID != "dolthub" {
		t.Errorf("expected dolthub, got %s", key.IntegrationID)
	}
}
