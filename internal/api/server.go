// Package api provides the HTTP REST server for the Wasteland wanted board.
//
// It wraps sdk.Client to expose browse, detail, dashboard, mutation, and branch
// operations as JSON endpoints consumable by any HTTP client.
package api

import (
	"net/http"

	"github.com/julianknutsen/wasteland/internal/sdk"
)

// Server is the HTTP API server wrapping an SDK client.
type Server struct {
	client *sdk.Client
	mux    *http.ServeMux
}

// New creates a Server backed by the given SDK client.
func New(client *sdk.Client) *Server {
	s := &Server{
		client: client,
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
