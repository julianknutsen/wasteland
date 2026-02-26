package hosted

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/julianknutsen/wasteland/internal/api"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

// Server provides hosted-mode handler composition.
type Server struct {
	resolver      *ClientResolver
	sessions      *SessionStore
	nango         *NangoClient
	sessionSecret string
}

// NewServer creates a hosted Server.
func NewServer(resolver *ClientResolver, sessions *SessionStore, nango *NangoClient, sessionSecret string) *Server {
	return &Server{
		resolver:      resolver,
		sessions:      sessions,
		nango:         nango,
		sessionSecret: sessionSecret,
	}
}

// Handler composes the hosted endpoints with the API server and static assets.
func (s *Server) Handler(apiServer *api.Server, assets fs.FS) http.Handler {
	mux := http.NewServeMux()

	// Health check for Railway / load balancers.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Auth endpoints (no auth middleware required).
	mux.HandleFunc("POST /api/auth/connect", s.handleConnect)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /api/auth/connect-session", s.handleConnectSession)

	// All other routes go through auth middleware → SPA handler.
	mux.Handle("/", s.AuthMiddleware(api.SPAHandler(apiServer, assets)))

	return mux
}

// connectRequest is the JSON body for POST /api/auth/connect.
type connectRequest struct {
	ConnectionID string `json:"connection_id"`
	RigHandle    string `json:"rig_handle"`
	ForkOrg      string `json:"fork_org"`
	ForkDB       string `json:"fork_db"`
	Upstream     string `json:"upstream"`
	Mode         string `json:"mode"`
}

// handleConnect is called after the frontend completes Nango auth.
// It writes the user config as Nango connection metadata, creates a session,
// and sets the session cookie.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.ConnectionID == "" || req.RigHandle == "" || req.ForkOrg == "" || req.ForkDB == "" || req.Upstream == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "connection_id, rig_handle, fork_org, fork_db, and upstream are required"})
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "wild-west"
	}

	// Write user config to Nango connection metadata.
	cfg := &UserConfig{
		RigHandle: req.RigHandle,
		ForkOrg:   req.ForkOrg,
		ForkDB:    req.ForkDB,
		Upstream:  req.Upstream,
		Mode:      mode,
	}
	if err := s.nango.SetMetadata(req.ConnectionID, cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	// Create session.
	sessionID := s.sessions.Create(req.ConnectionID)
	SetSessionCookie(w, sessionID, s.sessionSecret)

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

// authStatusResponse is the JSON response for GET /api/auth/status.
type authStatusResponse struct {
	Authenticated bool        `json:"authenticated"`
	Connected     bool        `json:"connected"`
	Config        *UserConfig `json:"config,omitempty"`
}

// handleAuthStatus returns the current session state.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := ReadSessionCookie(r, s.sessionSecret)
	if !ok {
		writeJSON(w, http.StatusOK, authStatusResponse{})
		return
	}

	session, ok := s.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusOK, authStatusResponse{})
		return
	}

	if session.ConnectionID == "" {
		writeJSON(w, http.StatusOK, authStatusResponse{Authenticated: true})
		return
	}

	// Fetch config from Nango.
	_, cfg, err := s.nango.GetConnection(session.ConnectionID)
	if err != nil {
		// Nango call failed — report as not connected so frontend can re-auth.
		writeJSON(w, http.StatusOK, authStatusResponse{Authenticated: true, Connected: false})
		return
	}

	writeJSON(w, http.StatusOK, authStatusResponse{
		Authenticated: true,
		Connected:     true,
		Config:        cfg,
	})
}

// handleLogout destroys the session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := ReadSessionCookie(r, s.sessionSecret)
	if ok {
		s.sessions.Delete(sessionID)
	}
	ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// connectSessionRequest is the JSON body for POST /api/auth/connect-session.
type connectSessionRequest struct {
	EndUserID string `json:"end_user_id"`
}

// connectSessionResponse is the JSON response for POST /api/auth/connect-session.
type connectSessionResponse struct {
	Token         string `json:"token"`
	IntegrationID string `json:"integration_id"`
}

// handleConnectSession creates a Nango connect session token for the frontend SDK.
func (s *Server) handleConnectSession(w http.ResponseWriter, r *http.Request) {
	var req connectSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.EndUserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "end_user_id is required"})
		return
	}

	token, err := s.nango.CreateConnectSession(req.EndUserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, connectSessionResponse{
		Token:         token,
		IntegrationID: s.nango.integrationID,
	})
}

// NewClientFunc returns a ClientFunc that reads the client from request context.
// This bridges the hosted auth middleware with api.Server's ClientFunc pattern.
func NewClientFunc() api.ClientFunc {
	return func(r *http.Request) (*sdk.Client, error) {
		client, ok := ClientFromContext(r.Context())
		if !ok {
			return nil, errNotAuthenticated
		}
		return client, nil
	}
}

var errNotAuthenticated = &authError{"not authenticated"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

// writeJSON writes a JSON response (duplicated here to avoid circular import
// with the api package, which provides the canonical version).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
