package federation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseUpstream(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOrg   string
		wantDB    string
		wantError bool
	}{
		{"valid", "steveyegge/wl-commons", "steveyegge", "wl-commons", false},
		{"valid with hyphens", "alice-dev/wl-commons", "alice-dev", "wl-commons", false},
		{"no slash", "wl-commons", "", "", true},
		{"empty org", "/wl-commons", "", "", true},
		{"empty db", "steveyegge/", "", "", true},
		{"empty", "", "", "", true},
		{"multiple slashes", "a/b/c", "a", "b/c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, db, err := ParseUpstream(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseUpstream(%q) expected error, got org=%q db=%q", tt.input, org, db)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseUpstream(%q) unexpected error: %v", tt.input, err)
				return
			}
			if org != tt.wantOrg {
				t.Errorf("ParseUpstream(%q) org = %q, want %q", tt.input, org, tt.wantOrg)
			}
			if db != tt.wantDB {
				t.Errorf("ParseUpstream(%q) db = %q, want %q", tt.input, db, tt.wantDB)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Upstream:  "steveyegge/wl-commons",
		ForkOrg:   "alice-dev",
		ForkDB:    "wl-commons",
		LocalDir:  "/tmp/test/wl-commons",
		RigHandle: "alice-dev",
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.Upstream != cfg.Upstream {
		t.Errorf("Upstream = %q, want %q", loaded.Upstream, cfg.Upstream)
	}
	if loaded.ForkOrg != cfg.ForkOrg {
		t.Errorf("ForkOrg = %q, want %q", loaded.ForkOrg, cfg.ForkOrg)
	}
	if loaded.ForkDB != cfg.ForkDB {
		t.Errorf("ForkDB = %q, want %q", loaded.ForkDB, cfg.ForkDB)
	}
	if loaded.RigHandle != cfg.RigHandle {
		t.Errorf("RigHandle = %q, want %q", loaded.RigHandle, cfg.RigHandle)
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig expected error for missing config")
	}
}

func TestEscapeSQLString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"it's", "it''s"},
		{"it''s", "it''''s"},
		{"", ""},
	}
	for _, tt := range tests {
		got := escapeSQLString(tt.input)
		if got != tt.want {
			t.Errorf("escapeSQLString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestForkDoltHubRepo(t *testing.T) {
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

			err := ForkDoltHubRepo("steveyegge", "wl-commons", "alice-dev", "test-token")
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLocalCloneDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-test")
	got := LocalCloneDir("steveyegge", "wl-commons")
	want := filepath.Join("/tmp/xdg-test", "wasteland", "steveyegge", "wl-commons")
	if got != want {
		t.Errorf("LocalCloneDir = %q, want %q", got, want)
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	got := ConfigPath()
	want := filepath.Join("/tmp/xdg-test", "wasteland", "config.json")
	if got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}

func TestConfigPath_DefaultXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := ConfigPath()
	want := filepath.Join(home, ".config", "wasteland", "config.json")
	if got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}
