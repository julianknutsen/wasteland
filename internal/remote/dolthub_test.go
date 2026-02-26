package remote

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoltHubProvider_ForkGraphQL(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantError  bool
	}{
		{"success", 200, `{"data":{"createFork":{"forkOperationName":"op-123"}}}`, false},
		{"already exists", 200, `{"errors":[{"message":"database has already been forked"}]}`, false},
		{"forbidden", 200, `{"errors":[{"message":"forbidden"}]}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST, got %s", r.Method)
				}

				// Verify session cookie.
				cookie := r.Header.Get("Cookie")
				if !strings.Contains(cookie, "dolthubToken=test-session-token") {
					t.Errorf("expected dolthubToken cookie, got %q", cookie)
				}

				// Verify GraphQL request body.
				var gqlReq graphqlRequest
				if err := json.NewDecoder(r.Body).Decode(&gqlReq); err != nil {
					t.Errorf("decoding request body: %v", err)
				}
				if !strings.Contains(gqlReq.Query, "createFork") {
					t.Errorf("query should contain createFork, got %q", gqlReq.Query)
				}
				vars := gqlReq.Variables
				if vars["parentOwnerName"] != "steveyegge" {
					t.Errorf("parentOwnerName = %q, want %q", vars["parentOwnerName"], "steveyegge")
				}
				if vars["parentRepoName"] != "wl-commons" {
					t.Errorf("parentRepoName = %q, want %q", vars["parentRepoName"], "wl-commons")
				}
				if vars["ownerName"] != "alice-dev" {
					t.Errorf("ownerName = %q, want %q", vars["ownerName"], "alice-dev")
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			oldURL := dolthubGraphQLURL
			dolthubGraphQLURL = server.URL
			defer func() { dolthubGraphQLURL = oldURL }()

			provider := NewDoltHubProvider("api-token")
			err := provider.forkGraphQL("steveyegge", "wl-commons", "alice-dev", "test-session-token")
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDoltHubProvider_ForkGraphQL_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	oldURL := dolthubGraphQLURL
	dolthubGraphQLURL = server.URL
	defer func() { dolthubGraphQLURL = oldURL }()

	provider := NewDoltHubProvider("api-token")
	err := provider.forkGraphQL("org", "db", "fork-org", "session-token")
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestDoltHubProvider_ForkDispatch_WithSessionToken(t *testing.T) {
	// When DOLTHUB_SESSION_TOKEN is set, Fork should use GraphQL.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"createFork":{"forkOperationName":"op-1"}}}`))
	}))
	defer server.Close()

	oldURL := dolthubGraphQLURL
	dolthubGraphQLURL = server.URL
	defer func() { dolthubGraphQLURL = oldURL }()

	t.Setenv("DOLTHUB_SESSION_TOKEN", "my-session")

	provider := NewDoltHubProvider("api-token")
	err := provider.Fork("org", "db", "fork-org")
	if err != nil {
		t.Errorf("Fork with session token: %v", err)
	}
}

func TestDoltHubProvider_ForkREST_Success(t *testing.T) {
	// REST fork: POST returns operation_name, poll returns success.
	pollCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "token api-token" {
			t.Errorf("expected auth header, got %q", r.Header.Get("authorization"))
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decoding request: %v", err)
			}
			if body["ownerName"] != "alice-dev" {
				t.Errorf("ownerName = %q, want %q", body["ownerName"], "alice-dev")
			}
			if body["parentOwnerName"] != "steveyegge" {
				t.Errorf("parentOwnerName = %q, want %q", body["parentOwnerName"], "steveyegge")
			}
			if body["parentDatabaseName"] != "wl-commons" {
				t.Errorf("parentDatabaseName = %q, want %q", body["parentDatabaseName"], "wl-commons")
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"Success","operation_name":"fork-op-123"}`))
			return
		}
		if r.Method == "GET" && r.URL.Query().Get("operationName") == "fork-op-123" {
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"status":"Pending"}`))
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"owner_name":"alice-dev","database_name":"wl-commons"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	provider := NewDoltHubProvider("api-token")
	err := provider.forkREST("steveyegge", "wl-commons", "alice-dev")
	if err != nil {
		t.Errorf("forkREST should succeed: %v", err)
	}
}

func TestDoltHubProvider_ForkREST_PollStatusSuccess(t *testing.T) {
	// REST fork: poll returns status "Success" without owner_name/database_name.
	pollCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"Success","operation_name":"fork-op-456"}`))
			return
		}
		if r.Method == "GET" && r.URL.Query().Get("operationName") == "fork-op-456" {
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"status":"Pending"}`))
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"Success"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	provider := NewDoltHubProvider("api-token")
	err := provider.forkREST("steveyegge", "wl-commons", "alice-dev")
	if err != nil {
		t.Errorf("forkREST should succeed on status-based completion: %v", err)
	}
}

func TestDoltHubProvider_ForkREST_AlreadyExists(t *testing.T) {
	// REST fork: POST returns "already exists" error → treated as success.
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"status":"Error","message":"database already exists"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	provider := NewDoltHubProvider("api-token")
	err := provider.forkREST("steveyegge", "wl-commons", "alice-dev")
	if err != nil {
		t.Errorf("forkREST should succeed for already-exists: %v", err)
	}
}

func TestDoltHubProvider_ForkREST_AuthError(t *testing.T) {
	// REST fork: auth error → falls back to exists-check → ForkRequiredError.
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"status":"Error","message":"unauthorized"}`))
			return
		}
		// Exists-check for fallback: fork doesn't exist.
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"query_execution_status":"Error"}`))
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	provider := NewDoltHubProvider("bad-token")
	err := provider.forkREST("steveyegge", "wl-commons", "alice-dev")
	if err == nil {
		t.Fatal("expected ForkRequiredError, got nil")
	}
	var forkErr *ForkRequiredError
	if !errors.As(err, &forkErr) {
		t.Fatalf("expected ForkRequiredError, got %T: %v", err, err)
	}
}

func TestDoltHubProvider_Fork_NoSession_UsesREST(t *testing.T) {
	// When no session token, Fork dispatches to forkREST (not ForkRequiredError).
	gotRESTFork := false
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			gotRESTFork = true
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"Success","operation_name":""}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	t.Setenv("DOLTHUB_SESSION_TOKEN", "")

	provider := NewDoltHubProvider("api-token")
	err := provider.Fork("steveyegge", "wl-commons", "alice-dev")
	if err != nil {
		t.Errorf("Fork should succeed via REST: %v", err)
	}
	if !gotRESTFork {
		t.Error("expected Fork to use REST API, but no POST /fork was received")
	}
}

func TestDoltHubProvider_Fork_NoSession_ForkExists(t *testing.T) {
	// REST fork fails with auth error, but fork already exists → success.
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"status":"Error","message":"forbidden"}`))
			return
		}
		// Exists-check fallback: fork exists.
		if r.Header.Get("authorization") != "token api-token" {
			t.Errorf("expected auth header, got %q", r.Header.Get("authorization"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"query_execution_status":"Success"}`))
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	t.Setenv("DOLTHUB_SESSION_TOKEN", "")

	provider := NewDoltHubProvider("api-token")
	err := provider.Fork("upstream-org", "wl-commons", "my-fork-org")
	if err != nil {
		t.Errorf("Fork should succeed when fork exists: %v", err)
	}
}

func TestDoltHubProvider_Fork_NoSession_ForkNotFound(t *testing.T) {
	// REST fork fails, fork doesn't exist → ForkRequiredError.
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/fork") {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"status":"Error","message":"forbidden"}`))
			return
		}
		// Exists-check fallback: fork not found.
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"query_execution_status":"Error","query_execution_message":"no such repository"}`))
	}))
	defer apiServer.Close()

	oldAPI := dolthubAPIBase
	dolthubAPIBase = apiServer.URL
	defer func() { dolthubAPIBase = oldAPI }()

	t.Setenv("DOLTHUB_SESSION_TOKEN", "")

	provider := NewDoltHubProvider("api-token")
	err := provider.Fork("hop", "wl-commons", "my-fork-org")
	if err == nil {
		t.Fatal("expected ForkRequiredError, got nil")
	}

	var forkErr *ForkRequiredError
	if !errors.As(err, &forkErr) {
		t.Fatalf("expected ForkRequiredError, got %T: %v", err, err)
	}
	if forkErr.UpstreamOrg != "hop" {
		t.Errorf("UpstreamOrg = %q, want %q", forkErr.UpstreamOrg, "hop")
	}
	if forkErr.UpstreamDB != "wl-commons" {
		t.Errorf("UpstreamDB = %q, want %q", forkErr.UpstreamDB, "wl-commons")
	}
	if forkErr.ForkOrg != "my-fork-org" {
		t.Errorf("ForkOrg = %q, want %q", forkErr.ForkOrg, "my-fork-org")
	}
}

func TestForkRequiredError_ForkURL(t *testing.T) {
	err := &ForkRequiredError{UpstreamOrg: "hop", UpstreamDB: "wl-commons", ForkOrg: "alice"}
	want := "https://www.dolthub.com/repositories/hop/wl-commons"
	if got := err.ForkURL(); got != want {
		t.Errorf("ForkURL() = %q, want %q", got, want)
	}
}

func TestDoltHubProvider_DatabaseURL(t *testing.T) {
	provider := NewDoltHubProvider("token")
	got := provider.DatabaseURL("steveyegge", "wl-commons")
	want := "https://doltremoteapi.dolthub.com/steveyegge/wl-commons"
	if got != want {
		t.Errorf("DatabaseURL = %q, want %q", got, want)
	}
}

func TestDoltHubProvider_Type(t *testing.T) {
	provider := NewDoltHubProvider("token")
	if got := provider.Type(); got != "dolthub" {
		t.Errorf("Type() = %q, want %q", got, "dolthub")
	}
}

func TestDoltHubProvider_ListPendingWantedIDs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/upstream-org/wl-commons/pulls", func(w http.ResponseWriter, r *http.Request) {
		// If it's a request for a specific pull, serve the detail.
		if strings.Contains(r.URL.Path, "/pulls/") {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pulls": []map[string]any{
				{"pull_id": "1", "state": "open"},
				{"pull_id": "2", "state": "open"},
				{"pull_id": "3", "state": "closed"},
			},
		})
	})
	mux.HandleFunc("/upstream-org/wl-commons/pulls/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"from_branch": "wl/alice/fix-login",
		})
	})
	mux.HandleFunc("/upstream-org/wl-commons/pulls/2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"from_branch": "wl/bob/add-feature",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	dolthubAPIBase = server.URL

	provider := NewDoltHubProvider("token")
	ids, err := provider.ListPendingWantedIDs("upstream-org", "wl-commons")
	if err != nil {
		t.Fatalf("ListPendingWantedIDs() error: %v", err)
	}

	if !ids["fix-login"] {
		t.Errorf("expected fix-login in pending IDs")
	}
	if !ids["add-feature"] {
		t.Errorf("expected add-feature in pending IDs")
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 pending IDs, got %d", len(ids))
	}
}

func TestDoltHubProvider_ListPendingWantedIDs_SkipsNonWLBranches(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/org/db/pulls", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/pulls/") {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pulls": []map[string]any{
				{"pull_id": "1", "state": "open"},
			},
		})
	})
	mux.HandleFunc("/org/db/pulls/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"from_branch": "feature/other-work",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	dolthubAPIBase = server.URL

	provider := NewDoltHubProvider("token")
	ids, err := provider.ListPendingWantedIDs("org", "db")
	if err != nil {
		t.Fatalf("ListPendingWantedIDs() error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 pending IDs for non-wl branches, got %d", len(ids))
	}
}
