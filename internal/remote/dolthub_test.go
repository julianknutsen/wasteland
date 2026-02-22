package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoltHubProvider_Fork(t *testing.T) {
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

				// Verify auth header.
				if r.Header.Get("authorization") != "token test-token" {
					t.Errorf("expected auth header, got %q", r.Header.Get("authorization"))
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

			provider := NewDoltHubProvider("test-token")
			err := provider.Fork("steveyegge", "wl-commons", "alice-dev")
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDoltHubProvider_Fork_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	oldURL := dolthubGraphQLURL
	dolthubGraphQLURL = server.URL
	defer func() { dolthubGraphQLURL = oldURL }()

	provider := NewDoltHubProvider("test-token")
	err := provider.Fork("org", "db", "fork-org")
	if err == nil {
		t.Error("expected error for HTTP 500")
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
