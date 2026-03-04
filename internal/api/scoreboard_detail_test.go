package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julianknutsen/wasteland/internal/commons"
)

func TestScoreboardDetail_Handler(t *testing.T) {
	db := newFakeDB()
	db.results = map[string]string{
		"GROUP BY s.subject":           "subject,stamp_count,weighted_score,unique_towns,avg_quality,avg_reliability\nalice,4,15,3,4.2,3.8\n",
		"stamp_id IS NOT NULL":         "completed_by,completions\nalice,2\n",
		"s.skill_tags":                 "subject,skill_tags\n",
		"display_name\nFROM rigs":      "handle,display_name\nalice,Alice Chen\n",
		"registered_at":                "handle,registered_at,rig_type\nalice,2024-01-15,human\n",
		"GROUP BY subject, severity":   "subject,severity,cnt\nalice,root,1\nalice,branch,2\nalice,leaf,1\n",
		"ORDER BY subject, created_at": "subject,author,severity,quality,reliability,skill_tags,message,created_at\nalice,bob,root,5,4,,great,2024-06-01\n",
		"FROM completions c":           "completed_by,wanted_id,wanted_title,completed_at,validated_at\nalice,w-1,Fix bug,2024-06-01,2024-06-02\n",
		"FROM badges":                  "rig_handle,badge_type,awarded_at\nalice,pioneer,2024-03-01\n",
	}

	client := newTestClient(db)
	srv := New(client)
	cache := NewCachedEndpoint(newScoreboardDetailRefresh(db), time.Hour)
	srv.SetScoreboardDetail(cache)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/scoreboard/detail")
	if err != nil {
		t.Fatalf("GET /api/scoreboard/detail: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS Allow-Origin = %q, want *", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control = %q, want 'public, max-age=300'", got)
	}

	var result ScoreboardDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].RigHandle != "alice" {
		t.Errorf("rig_handle = %q, want alice", result.Entries[0].RigHandle)
	}
	if len(result.Entries[0].Stamps) != 1 {
		t.Errorf("stamps count = %d, want 1", len(result.Entries[0].Stamps))
	}
	if result.UpdatedAt == "" {
		t.Error("expected updated_at, got empty")
	}
}

func TestScoreboardDetail_NotConfigured(t *testing.T) {
	db := newFakeDB()
	client := newTestClient(db)
	srv := New(client)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/scoreboard/detail")
	if err != nil {
		t.Fatalf("GET /api/scoreboard/detail: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestScoreboardDetail_CORSPreflight(t *testing.T) {
	db := newFakeDB()
	client := newTestClient(db)
	srv := New(client)
	cache := NewCachedEndpoint(newScoreboardDetailRefresh(db), time.Hour)
	srv.SetScoreboardDetail(cache)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/scoreboard/detail", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /api/scoreboard/detail: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS Allow-Origin = %q, want *", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Errorf("CORS Allow-Methods = %q, want 'GET, OPTIONS'", got)
	}
}

// newScoreboardDetailRefresh creates the refresh function used in tests.
func newScoreboardDetailRefresh(db *fakeDB) func() ([]byte, error) {
	return func() ([]byte, error) {
		entries, err := commons.QueryScoreboardDetail(db, 100)
		if err != nil {
			return nil, err
		}
		return json.Marshal(toScoreboardDetailResponse(entries))
	}
}
