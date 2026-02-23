package federation

import (
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

	store := NewConfigStore()
	cfg := &Config{
		Upstream:  "steveyegge/wl-commons",
		ForkOrg:   "alice-dev",
		ForkDB:    "wl-commons",
		LocalDir:  "/tmp/test/wl-commons",
		RigHandle: "alice-dev",
	}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("steveyegge/wl-commons")
	if err != nil {
		t.Fatalf("Load: %v", err)
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

	// Verify file is in wastelands/{org}/{db}.json
	expectedPath := filepath.Join(tmpDir, "wasteland", "wastelands", "steveyegge", "wl-commons.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("config file not at expected path %s: %v", expectedPath, err)
	}
}

func TestConfigLoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	store := NewConfigStore()
	_, err := store.Load("nonexistent/db")
	if err == nil {
		t.Error("Load expected error for missing config")
	}
}

func TestFileConfigStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	store := NewConfigStore()

	// Empty at first.
	upstreams, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(upstreams) != 0 {
		t.Errorf("expected 0 upstreams, got %d", len(upstreams))
	}

	// Save two configs.
	if err = store.Save(&Config{Upstream: "org1/db1", ForkOrg: "fork1", ForkDB: "db1"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if err = store.Save(&Config{Upstream: "org2/db2", ForkOrg: "fork2", ForkDB: "db2"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	upstreams, err = store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(upstreams) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(upstreams))
	}

	found := map[string]bool{}
	for _, u := range upstreams {
		found[u] = true
	}
	if !found["org1/db1"] {
		t.Error("expected org1/db1 in list")
	}
	if !found["org2/db2"] {
		t.Error("expected org2/db2 in list")
	}
}

func TestFileConfigStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	store := NewConfigStore()

	// Save then delete.
	if err := store.Save(&Config{Upstream: "org1/db1", ForkOrg: "fork1", ForkDB: "db1"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if err := store.Delete("org1/db1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Should no longer be loadable.
	_, err := store.Load("org1/db1")
	if err == nil {
		t.Error("expected error loading deleted config")
	}

	// List should be empty.
	upstreams, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(upstreams) != 0 {
		t.Errorf("expected 0 upstreams after delete, got %d", len(upstreams))
	}

	// Delete of nonexistent should error.
	err = store.Delete("nonexistent/db")
	if err == nil {
		t.Error("expected error deleting nonexistent config")
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

func TestLocalCloneDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-test")
	got := LocalCloneDir("steveyegge", "wl-commons")
	want := filepath.Join("/tmp/xdg-test", "wasteland", "steveyegge", "wl-commons")
	if got != want {
		t.Errorf("LocalCloneDir = %q, want %q", got, want)
	}
}
