package hosted

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newFakeNangoForResolver(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &UserConfig{
		RigHandle: "alice",
		ForkOrg:   "alice-org",
		ForkDB:    "wl-commons",
		Upstream:  "wasteland/wl-commons",
		Mode:      "wild-west",
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer resolver-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" && r.URL.Path == "/connection/conn-1" {
			resp := nangoConnectionResponse{
				ConnectionID: "conn-1",
			}
			resp.Credentials.APIKey = "test-dolthub-token"
			b, _ := json.Marshal(cfg)
			resp.Metadata = json.RawMessage(b)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
}

func TestClientResolver_Resolve(t *testing.T) {
	ts := newFakeNangoForResolver(t)
	defer ts.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "resolver-secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewClientResolver(nango, sessions)

	session := &UserSession{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		CreatedAt:    time.Now(),
	}

	client, err := resolver.Resolve(session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.RigHandle() != "alice" {
		t.Errorf("expected alice, got %s", client.RigHandle())
	}
	if client.Mode() != "wild-west" {
		t.Errorf("expected wild-west, got %s", client.Mode())
	}
}

func TestClientResolver_CachesClient(t *testing.T) {
	ts := newFakeNangoForResolver(t)
	defer ts.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "resolver-secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewClientResolver(nango, sessions)

	session := &UserSession{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		CreatedAt:    time.Now(),
	}

	client1, err := resolver.Resolve(session)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}

	client2, err := resolver.Resolve(session)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}

	// Should get the same cached instance.
	if client1 != client2 {
		t.Error("expected same client instance from cache")
	}
}

func TestClientResolver_NoToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := nangoConnectionResponse{ConnectionID: "conn-1"}
		// No API key set.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewClientResolver(nango, sessions)

	session := &UserSession{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		CreatedAt:    time.Now(),
	}

	_, err := resolver.Resolve(session)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestClientResolver_NoConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := nangoConnectionResponse{ConnectionID: "conn-1"}
		resp.Credentials.APIKey = "token"
		// No metadata.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	nango := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
	})
	sessions := NewSessionStore()
	resolver := NewClientResolver(nango, sessions)

	session := &UserSession{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		CreatedAt:    time.Now(),
	}

	_, err := resolver.Resolve(session)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}
