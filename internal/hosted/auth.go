package hosted

import (
	"context"
	"net/http"
	"strings"

	"github.com/julianknutsen/wasteland/internal/sdk"
)

type contextKey string

const clientContextKey contextKey = "hosted-client"

// ClientFromContext extracts the sdk.Client injected by auth middleware.
func ClientFromContext(ctx context.Context) (*sdk.Client, bool) {
	client, ok := ctx.Value(clientContextKey).(*sdk.Client)
	return client, ok
}

// AuthMiddleware protects /api/* routes (excluding /api/auth/*).
// It resolves the session cookie, looks up the Nango connection, and injects
// the per-user sdk.Client into the request context.
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for /api/auth/* endpoints.
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Non-API routes (static files, SPA) don't need auth.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Read and verify session cookie.
		sessionID, ok := ReadSessionCookie(r, s.sessionSecret)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}

		session, ok := s.sessions.Get(sessionID)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
			return
		}

		if session.ConnectionID == "" {
			writeJSON(w, http.StatusPreconditionFailed, map[string]string{"error": "DoltHub not connected"})
			return
		}

		// Resolve the per-user sdk.Client.
		client, err := s.resolver.Resolve(session)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "failed to resolve client: " + err.Error()})
			return
		}

		// Inject client into context.
		ctx := context.WithValue(r.Context(), clientContextKey, client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
