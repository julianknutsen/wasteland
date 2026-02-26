package hosted

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakeNangoServer(t *testing.T, token string, metadata *UserConfig) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == "GET" && r.URL.Path == "/connection/conn-1":
			resp := nangoConnectionResponse{
				ConnectionID: "conn-1",
			}
			resp.Credentials.APIKey = token
			if metadata != nil {
				b, _ := json.Marshal(metadata)
				resp.Metadata = json.RawMessage(b)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == "PATCH" && r.URL.Path == "/connection/conn-1/metadata":
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func TestNangoClient_GetConnection(t *testing.T) {
	cfg := &UserConfig{
		RigHandle: "alice",
		ForkOrg:   "alice-org",
		ForkDB:    "wl-commons",
		Upstream:  "wasteland/wl-commons",
		Mode:      "pr",
	}
	ts := newFakeNangoServer(t, "dolthub-token-123", cfg)
	defer ts.Close()

	client := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "test-secret",
		IntegrationID: "dolthub",
	})

	token, userCfg, err := client.GetConnection("conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "dolthub-token-123" {
		t.Errorf("expected dolthub-token-123, got %s", token)
	}
	if userCfg == nil {
		t.Fatal("expected user config, got nil")
	}
	if userCfg.RigHandle != "alice" {
		t.Errorf("expected alice, got %s", userCfg.RigHandle)
	}
	if userCfg.Mode != "pr" {
		t.Errorf("expected pr, got %s", userCfg.Mode)
	}
}

func TestNangoClient_GetConnection_NoMetadata(t *testing.T) {
	ts := newFakeNangoServer(t, "dolthub-token-123", nil)
	defer ts.Close()

	client := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "test-secret",
		IntegrationID: "dolthub",
	})

	token, userCfg, err := client.GetConnection("conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "dolthub-token-123" {
		t.Errorf("expected dolthub-token-123, got %s", token)
	}
	if userCfg != nil {
		t.Errorf("expected nil config, got %+v", userCfg)
	}
}

func TestNangoClient_GetConnection_NotFound(t *testing.T) {
	ts := newFakeNangoServer(t, "", nil)
	defer ts.Close()

	client := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "test-secret",
		IntegrationID: "dolthub",
	})

	_, _, err := client.GetConnection("conn-missing")
	if err == nil {
		t.Fatal("expected error for missing connection")
	}
}

func TestNangoClient_SetMetadata(t *testing.T) {
	ts := newFakeNangoServer(t, "", nil)
	defer ts.Close()

	client := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "test-secret",
		IntegrationID: "dolthub",
	})

	err := client.SetMetadata("conn-1", &UserConfig{
		RigHandle: "bob",
		Mode:      "wild-west",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNangoClient_Unauthorized(t *testing.T) {
	ts := newFakeNangoServer(t, "", nil)
	defer ts.Close()

	client := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "wrong-secret",
		IntegrationID: "dolthub",
	})

	_, _, err := client.GetConnection("conn-1")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestNangoClient_Defaults(t *testing.T) {
	client := NewNangoClient(NangoConfig{
		SecretKey: "test",
	})
	if client.baseURL != "https://api.nango.dev" {
		t.Errorf("expected default base URL, got %s", client.baseURL)
	}
	if client.integrationID != "dolthub" {
		t.Errorf("expected default integration ID, got %s", client.integrationID)
	}
}
