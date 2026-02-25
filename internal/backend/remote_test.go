package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := DoltHubAPIBase
	DoltHubAPIBase = srv.URL
	return srv, func() {
		DoltHubAPIBase = old
		srv.Close()
	}
}

func TestRemoteDB_Query_Main(t *testing.T) {
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify path: /{readOwner}/{readDB}/main
		if !strings.Contains(r.URL.Path, "/upstream-org/wl-commons/main") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("authorization") != "token test-token" {
			t.Errorf("missing auth header")
		}
		resp := map[string]any{
			"query_execution_status": "Success",
			"schema_fragment": []map[string]string{
				{"columnName": "id", "columnType": "varchar(20)"},
				{"columnName": "status", "columnType": "varchar(20)"},
			},
			"rows": []map[string]string{
				{"id": "w-001", "status": "open"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	csv, err := db.Query("SELECT id, status FROM wanted", "")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), csv)
	}
	if lines[1] != "w-001,open" {
		t.Errorf("row = %q, want %q", lines[1], "w-001,open")
	}
}

func TestRemoteDB_Query_Branch(t *testing.T) {
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Branch refs should route to the fork database.
		if !strings.Contains(r.URL.Path, "/fork-org/wl-commons/wl/alice/w-001") {
			t.Errorf("unexpected path for branch query: %s", r.URL.Path)
		}
		resp := map[string]any{
			"query_execution_status": "Success",
			"schema_fragment": []map[string]string{
				{"columnName": "status", "columnType": "varchar(20)"},
			},
			"rows": []map[string]string{
				{"status": "claimed"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	csv, err := db.Query("SELECT status FROM wanted WHERE id='w-001'", "wl/alice/w-001")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	if !strings.Contains(csv, "claimed") {
		t.Errorf("expected 'claimed' in output, got: %q", csv)
	}
}

func TestRemoteDB_Exec(t *testing.T) {
	callCount := 0
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			// Verify write path includes branch.
			if !strings.Contains(r.URL.Path, "/fork-org/wl-commons/write/main/wl/alice/w-001") {
				t.Errorf("unexpected write path: %s", r.URL.Path)
			}
			resp := map[string]string{
				"query_execution_status": "Success",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		t.Errorf("unexpected method: %s", r.Method)
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	err := db.Exec("wl/alice/w-001", "wl claim: w-001", false,
		"UPDATE wanted SET status='claimed' WHERE id='w-001'")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
}

func TestRemoteDB_Exec_MainBranch(t *testing.T) {
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Empty branch should default to main.
		if !strings.Contains(r.URL.Path, "/write/main/main") {
			t.Errorf("expected write/main/main path, got: %s", r.URL.Path)
		}
		resp := map[string]string{
			"query_execution_status": "Success",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	err := db.Exec("", "wl claim: w-001", false,
		"UPDATE wanted SET status='claimed' WHERE id='w-001'")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
}

func TestRemoteDB_Branches(t *testing.T) {
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"query_execution_status": "Success",
			"schema_fragment": []map[string]string{
				{"columnName": "name", "columnType": "varchar(255)"},
			},
			"rows": []map[string]string{
				{"name": "wl/alice/w-001"},
				{"name": "wl/alice/w-002"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	branches, err := db.Branches("wl/alice/")
	if err != nil {
		t.Fatalf("Branches error: %v", err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}
	if branches[0] != "wl/alice/w-001" {
		t.Errorf("branch[0] = %q, want %q", branches[0], "wl/alice/w-001")
	}
}

func TestRemoteDB_NoOps(t *testing.T) {
	t.Parallel()
	db := NewRemoteDB("token", "up", "db", "fork", "db")

	if err := db.PushBranch("branch", nil); err != nil {
		t.Errorf("PushBranch should be no-op, got: %v", err)
	}
	if err := db.PushMain(nil); err != nil {
		t.Errorf("PushMain should be no-op, got: %v", err)
	}
	if err := db.Sync(); err != nil {
		t.Errorf("Sync should be no-op, got: %v", err)
	}
	if err := db.MergeBranch("branch"); err != nil {
		t.Errorf("MergeBranch should be no-op, got: %v", err)
	}
	if err := db.DeleteRemoteBranch("branch"); err != nil {
		t.Errorf("DeleteRemoteBranch should be no-op, got: %v", err)
	}
	if err := db.PushWithSync(nil); err != nil {
		t.Errorf("PushWithSync should be no-op, got: %v", err)
	}
}

func TestRemoteDB_Exec_Poll(t *testing.T) {
	pollCount := 0
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			resp := map[string]string{
				"operation_name": "op-123",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// GET = poll
		pollCount++
		if pollCount < 2 {
			resp := map[string]string{
				"query_execution_status": "Running",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp := map[string]string{
			"query_execution_status": "Success",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	err := db.Exec("main", "test", false, "UPDATE wanted SET status='open'")
	if err != nil {
		t.Fatalf("Exec with poll error: %v", err)
	}
	if pollCount < 2 {
		t.Errorf("expected at least 2 polls, got %d", pollCount)
	}
}

func TestRemoteDB_Query_Error(t *testing.T) {
	srv, cleanup := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal server error")
	})
	defer cleanup()

	db := NewRemoteDB("test-token", "upstream-org", "wl-commons", "fork-org", "wl-commons")
	db.client = srv.Client()

	_, err := db.Query("SELECT 1", "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
