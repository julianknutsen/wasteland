// Package api provides the HTTP REST server for the Wasteland wanted board.
//
// It wraps sdk.Client to expose browse, detail, dashboard, mutation, and branch
// operations as JSON endpoints consumable by any HTTP client.
package api

import (
	"net/http"

	"github.com/julianknutsen/wasteland/internal/sdk"
)

// ClientFunc resolves an sdk.Client from an HTTP request. In self-sovereign mode
// this returns a static client; in hosted mode it resolves per-user from session.
type ClientFunc func(r *http.Request) (*sdk.Client, error)

// WorkspaceFunc resolves an sdk.Workspace from an HTTP request.
type WorkspaceFunc func(r *http.Request) (*sdk.Workspace, error)

// Server is the HTTP API server wrapping an SDK client.
type Server struct {
	clientFunc    ClientFunc
	workspaceFunc WorkspaceFunc
	mux           *http.ServeMux
	hosted        bool // true when running in multi-tenant hosted mode
}

// New creates a Server backed by the given SDK client.
// This is the backwards-compatible constructor for self-sovereign mode.
func New(client *sdk.Client) *Server {
	return NewWithClientFunc(func(_ *http.Request) (*sdk.Client, error) {
		return client, nil
	})
}

// NewHosted creates a Server for multi-tenant hosted mode.
func NewHosted(fn ClientFunc) *Server {
	s := &Server{
		clientFunc: fn,
		mux:        http.NewServeMux(),
		hosted:     true,
	}
	s.registerRoutes()
	return s
}

// NewHostedWorkspace creates a Server for multi-tenant hosted mode with workspace support.
func NewHostedWorkspace(clientFn ClientFunc, workspaceFn WorkspaceFunc) *Server {
	s := &Server{
		clientFunc:    clientFn,
		workspaceFunc: workspaceFn,
		mux:           http.NewServeMux(),
		hosted:        true,
	}
	s.registerRoutes()
	return s
}

// NewWithClientFunc creates a Server that resolves a client per-request.
func NewWithClientFunc(fn ClientFunc) *Server {
	s := &Server{
		clientFunc: fn,
		mux:        http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
