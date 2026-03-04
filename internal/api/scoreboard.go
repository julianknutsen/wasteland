package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/julianknutsen/wasteland/internal/commons"
)

// ScoreboardResponse is the JSON response for GET /api/scoreboard.
type ScoreboardResponse struct {
	Entries   []ScoreboardEntryJSON `json:"entries"`
	UpdatedAt string                `json:"updated_at"`
}

// ScoreboardEntryJSON is the JSON representation of a scoreboard entry.
type ScoreboardEntryJSON struct {
	RigHandle      string   `json:"rig_handle"`
	DisplayName    string   `json:"display_name,omitempty"`
	TrustTier      string   `json:"trust_tier"`
	StampCount     int      `json:"stamp_count"`
	WeightedScore  int      `json:"weighted_score"`
	UniqueTowns    int      `json:"unique_towns"`
	Completions    int      `json:"completions"`
	AvgQuality     float64  `json:"avg_quality"`
	AvgReliability float64  `json:"avg_reliability"`
	AvgCreativity  float64  `json:"avg_creativity"`
	TopSkills      []string `json:"top_skills,omitempty"`
}

// CachedEndpoint manages a cached, periodically refreshed JSON endpoint.
type CachedEndpoint struct {
	mu        sync.RWMutex
	cached    []byte // pre-serialized JSON
	updatedAt time.Time
	refreshFn func() ([]byte, error)
	interval  time.Duration
	done      chan struct{}
}

// NewCachedEndpoint creates a new cached endpoint with a generic refresh callback.
func NewCachedEndpoint(refreshFn func() ([]byte, error), interval time.Duration) *CachedEndpoint {
	return &CachedEndpoint{
		refreshFn: refreshFn,
		interval:  interval,
		done:      make(chan struct{}),
	}
}

// NewScoreboardCache creates a CachedEndpoint that refreshes scoreboard data.
func NewScoreboardCache(db commons.DB, interval time.Duration) *CachedEndpoint {
	return NewCachedEndpoint(func() ([]byte, error) {
		entries, err := commons.QueryScoreboard(db, 100)
		if err != nil {
			return nil, err
		}
		return json.Marshal(toScoreboardResponse(entries))
	}, interval)
}

// Start begins the background refresh goroutine.
func (ce *CachedEndpoint) Start() {
	go ce.run()
}

// Stop halts the background refresh goroutine.
func (ce *CachedEndpoint) Stop() {
	close(ce.done)
}

func (ce *CachedEndpoint) run() {
	// Initial load.
	ce.refresh()

	ticker := time.NewTicker(ce.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ce.done:
			return
		case <-ticker.C:
			ce.refresh()
		}
	}
}

func (ce *CachedEndpoint) refresh() {
	data, err := ce.refreshFn()
	if err != nil {
		slog.Warn("cached endpoint refresh failed", "error", err)
		return
	}

	ce.mu.Lock()
	ce.cached = data
	ce.updatedAt = time.Now().UTC()
	ce.mu.Unlock()
}

// Get returns the cached JSON. If the cache is empty, triggers a synchronous load.
func (ce *CachedEndpoint) Get() []byte {
	ce.mu.RLock()
	data := ce.cached
	ce.mu.RUnlock()

	if data != nil {
		return data
	}

	// First request: synchronous load.
	ce.refresh()

	ce.mu.RLock()
	data = ce.cached
	ce.mu.RUnlock()
	return data
}

func toScoreboardResponse(entries []commons.ScoreboardEntry) *ScoreboardResponse {
	items := make([]ScoreboardEntryJSON, len(entries))
	for i, e := range entries {
		items[i] = ScoreboardEntryJSON{
			RigHandle:      e.RigHandle,
			DisplayName:    e.DisplayName,
			TrustTier:      e.TrustTier,
			StampCount:     e.StampCount,
			WeightedScore:  e.WeightedScore,
			UniqueTowns:    e.UniqueTowns,
			Completions:    e.Completions,
			AvgQuality:     e.AvgQuality,
			AvgReliability: e.AvgReliab,
			AvgCreativity:  e.AvgCreativity,
			TopSkills:      e.TopSkills,
		}
	}
	return &ScoreboardResponse{
		Entries:   items,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// handleScoreboard serves the cached scoreboard JSON with CORS headers.
func (s *Server) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	// CORS headers for public access.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if s.scoreboard == nil {
		writeError(w, http.StatusServiceUnavailable, "scoreboard not configured")
		return
	}

	data := s.scoreboard.Get()
	if data == nil {
		writeError(w, http.StatusServiceUnavailable, "scoreboard data unavailable")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
