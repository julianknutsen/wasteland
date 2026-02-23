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

func TestDoltHubProvider_Fork_NoSession_ForkExists(t *testing.T) {
	// When fork database already exists on DoltHub, Fork returns nil.
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// When fork database does not exist, Fork returns ForkRequiredError.
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
