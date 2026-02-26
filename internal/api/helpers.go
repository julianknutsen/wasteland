package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julianknutsen/wasteland/internal/commons"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

// decodeJSON reads the request body as JSON into v.
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close() //nolint:errcheck // best-effort close
	return json.NewDecoder(r.Body).Decode(v)
}

// parseIntParam parses a query parameter as an integer with a default value.
func parseIntParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// parseQueryFilter extracts browse filter parameters from the request query string.
func parseQueryFilter(r *http.Request) commons.BrowseFilter {
	q := r.URL.Query()

	sort := commons.SortPriority
	switch q.Get("sort") {
	case "newest":
		sort = commons.SortNewest
	case "alpha":
		sort = commons.SortAlpha
	}

	view := q.Get("view")
	if view == "" {
		view = "mine"
	}

	return commons.BrowseFilter{
		Status:   q.Get("status"),
		Project:  q.Get("project"),
		Type:     q.Get("type"),
		Priority: parseIntParam(r, "priority", -1),
		Limit:    parseIntParam(r, "limit", 50),
		Search:   q.Get("search"),
		Sort:     sort,
		View:     view,
	}
}
