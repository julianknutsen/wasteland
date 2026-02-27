package hosted

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakeNangoServer(t *testing.T, token string, metadata *UserMetadata) *httptest.Server {
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
	meta := &UserMetadata{
		RigHandle: "alice",
		Wastelands: []WastelandConfig{
			{
				Upstream: "wasteland/wl-commons",
				ForkOrg:  "alice-org",
				ForkDB:   "wl-commons",
				Mode:     "pr",
			},
		},
	}
	ts := newFakeNangoServer(t, "dolthub-token-123", meta)
	defer ts.Close()

	client := NewNangoClient(NangoConfig{
		BaseURL:       ts.URL,
		SecretKey:     "test-secret",
		IntegrationID: "dolthub",
	})

	token, userMeta, err := client.GetConnection("conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "dolthub-token-123" {
		t.Errorf("expected dolthub-token-123, got %s", token)
	}
	if userMeta == nil {
		t.Fatal("expected user metadata, got nil")
	}
	if userMeta.RigHandle != "alice" {
		t.Errorf("expected alice, got %s", userMeta.RigHandle)
	}
	if len(userMeta.Wastelands) != 1 {
		t.Fatalf("expected 1 wasteland, got %d", len(userMeta.Wastelands))
	}
	if userMeta.Wastelands[0].Mode != "pr" {
		t.Errorf("expected pr, got %s", userMeta.Wastelands[0].Mode)
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

	token, userMeta, err := client.GetConnection("conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "dolthub-token-123" {
		t.Errorf("expected dolthub-token-123, got %s", token)
	}
	if userMeta != nil {
		t.Errorf("expected nil metadata, got %+v", userMeta)
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

	err := client.SetMetadata("conn-1", &UserMetadata{
		RigHandle: "bob",
		Wastelands: []WastelandConfig{
			{Upstream: "wasteland/wl-commons", Mode: "wild-west"},
		},
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

func TestParseMetadata_NewFormat(t *testing.T) {
	raw := json.RawMessage(`{
		"rig_handle": "alice",
		"wastelands": [
			{"upstream": "hop/wl-commons", "fork_org": "alice-org", "fork_db": "wl-commons", "mode": "wild-west"},
			{"upstream": "julianknutsen/gascity", "fork_org": "alice-org", "fork_db": "gascity", "mode": "pr"}
		]
	}`)
	meta := parseMetadata(raw)
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if meta.RigHandle != "alice" {
		t.Errorf("expected alice, got %s", meta.RigHandle)
	}
	if len(meta.Wastelands) != 2 {
		t.Fatalf("expected 2 wastelands, got %d", len(meta.Wastelands))
	}
	if meta.Wastelands[0].Upstream != "hop/wl-commons" {
		t.Errorf("expected hop/wl-commons, got %s", meta.Wastelands[0].Upstream)
	}
}

func TestParseMetadata_LegacyFormat(t *testing.T) {
	raw := json.RawMessage(`{
		"rig_handle": "bob",
		"fork_org": "bob-org",
		"fork_db": "wl-commons",
		"upstream": "wasteland/wl-commons",
		"mode": "pr",
		"signing": true
	}`)
	meta := parseMetadata(raw)
	if meta == nil {
		t.Fatal("expected non-nil metadata from legacy format")
	}
	if meta.RigHandle != "bob" {
		t.Errorf("expected bob, got %s", meta.RigHandle)
	}
	if len(meta.Wastelands) != 1 {
		t.Fatalf("expected 1 wasteland from migration, got %d", len(meta.Wastelands))
	}
	wl := meta.Wastelands[0]
	if wl.Upstream != "wasteland/wl-commons" {
		t.Errorf("expected wasteland/wl-commons, got %s", wl.Upstream)
	}
	if wl.ForkOrg != "bob-org" {
		t.Errorf("expected bob-org, got %s", wl.ForkOrg)
	}
	if wl.Mode != "pr" {
		t.Errorf("expected pr, got %s", wl.Mode)
	}
	if !wl.Signing {
		t.Error("expected signing=true")
	}
}

func TestParseMetadata_Null(t *testing.T) {
	meta := parseMetadata(json.RawMessage("null"))
	if meta != nil {
		t.Errorf("expected nil, got %+v", meta)
	}
}

func TestParseMetadata_Empty(t *testing.T) {
	meta := parseMetadata(json.RawMessage(""))
	if meta != nil {
		t.Errorf("expected nil, got %+v", meta)
	}
}

func TestUserMetadata_FindWasteland(t *testing.T) {
	meta := &UserMetadata{
		Wastelands: []WastelandConfig{
			{Upstream: "a/repo", Mode: "wild-west"},
			{Upstream: "b/repo", Mode: "pr"},
		},
	}

	wl := meta.FindWasteland("b/repo")
	if wl == nil {
		t.Fatal("expected to find b/repo")
	}
	if wl.Mode != "pr" {
		t.Errorf("expected pr, got %s", wl.Mode)
	}

	if meta.FindWasteland("c/repo") != nil {
		t.Error("expected nil for missing upstream")
	}
}

func TestUserMetadata_UpsertWasteland(t *testing.T) {
	meta := &UserMetadata{
		Wastelands: []WastelandConfig{
			{Upstream: "a/repo", Mode: "wild-west"},
		},
	}

	// Insert new.
	meta.UpsertWasteland(WastelandConfig{Upstream: "b/repo", Mode: "pr"})
	if len(meta.Wastelands) != 2 {
		t.Fatalf("expected 2 wastelands after insert, got %d", len(meta.Wastelands))
	}

	// Update existing.
	meta.UpsertWasteland(WastelandConfig{Upstream: "a/repo", Mode: "pr"})
	if len(meta.Wastelands) != 2 {
		t.Fatalf("expected 2 wastelands after update, got %d", len(meta.Wastelands))
	}
	wl := meta.FindWasteland("a/repo")
	if wl.Mode != "pr" {
		t.Errorf("expected pr after upsert, got %s", wl.Mode)
	}
}

func TestUserMetadata_RemoveWasteland(t *testing.T) {
	meta := &UserMetadata{
		Wastelands: []WastelandConfig{
			{Upstream: "a/repo"},
			{Upstream: "b/repo"},
		},
	}

	if !meta.RemoveWasteland("a/repo") {
		t.Error("expected true for existing upstream")
	}
	if len(meta.Wastelands) != 1 {
		t.Fatalf("expected 1 wasteland after remove, got %d", len(meta.Wastelands))
	}

	if meta.RemoveWasteland("c/repo") {
		t.Error("expected false for missing upstream")
	}
}
