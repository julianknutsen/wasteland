package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoltHubProvider_Fork(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantError  bool
	}{
		{"success", 200, `{"status":"ok"}`, false},
		{"already exists", 409, `{"message":"already exists"}`, false},
		{"forbidden", 403, `{"message":"forbidden"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/database/fork" {
					t.Errorf("expected /database/fork, got %s", r.URL.Path)
				}
				if r.Header.Get("authorization") != "token test-token" {
					t.Errorf("expected auth header, got %q", r.Header.Get("authorization"))
				}

				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decoding request body: %v", err)
				}
				if body["from_owner"] != "steveyegge" {
					t.Errorf("from_owner = %q, want %q", body["from_owner"], "steveyegge")
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			oldBase := dolthubAPIBase
			dolthubAPIBase = server.URL
			defer func() { dolthubAPIBase = oldBase }()

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
